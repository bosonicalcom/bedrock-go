package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/bosonicalcom/bedrock-go/observability/logx"
	"github.com/bosonicalcom/bedrock-go/proc"
	"github.com/bosonicalcom/bedrock-go/syncx"
)

// - Sentinel implementation

// A Sentinel is a utility system component which performs health probes actively on registered Monitor(s) at configured intervals.
// It detects unhealthy states and reports them via error handling mechanisms (e.g., observability like traces/logs/metrics).
//
// It supports concurrent probes, error handling, and graceful shutdown.
//
// Zero-value can be used with default configuration.
type Sentinel struct {
	config   *SentinelConfig
	monitors map[string]Monitor
	mu       sync.RWMutex // protects monitors map before Start

	// lifecycle management
	started atomic.Bool
	stopped atomic.Bool

	// concurrency management
	rootProcessCtx       context.Context    // the context for the root process (lifecycle management)
	rootProcessCtxCancel context.CancelFunc // cancels rootProcessCtx to signal shutdown
	taskQueue            chan probeTask
	taskQueueDone        chan struct{}  // closed when taskQueue is closed, signals senders to stop
	inFlightProcWg       sync.WaitGroup // to wait for in-flight processes to finish (cycles + individual probes)
}

// NewSentinelWithConfig creates a new Sentinel with the provided configuration.
func NewSentinelWithConfig(config SentinelConfig) *Sentinel {
	return &Sentinel{
		config: &config,
	}
}

// NewSentinel creates a new Sentinel with the provided options.
func NewSentinel(options ...SentinelOption) *Sentinel {
	config := newDefaultSentinelConfig()
	for _, option := range options {
		option(config)
	}
	return &Sentinel{
		config: config,
	}
}

// Process returns a [proc.BackgroundProcess] that runs the Sentinel's Start and GracefulStop methods.
func (s *Sentinel) Process() proc.BackgroundProcess {
	return proc.BackgroundProcess{
		RunFunc: func(ctx context.Context) error {
			return s.Start()
		},
		ShutdownFunc: func(ctx context.Context) error {
			return s.GracefulStop()
		},
	}
}

// Register adds one or more Monitor(s) to the Sentinel for health checking.
//
// Register returns ErrAlreadyStarted if called after Start has been called.
// All monitors must be registered before calling Start.
//
// Register is safe to call concurrently before Start.
func (s *Sentinel) Register(monitors ...Monitor) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started.Load() {
		return ErrAlreadyStarted
	}
	if s.monitors == nil {
		s.monitors = make(map[string]Monitor, len(monitors))
	}
	for _, monitor := range monitors {
		s.monitors[monitor.Name()] = monitor
	}
	return nil
}

// ErrAlreadyStarted is returned when Start is called on a Sentinel that has already been started,
// or when Register is called after Start.
var ErrAlreadyStarted = errors.New("sentinel already started")

// Start begins the health check monitoring process. It runs indefinitely until a graceful stop is initiated.
//
// Start returns ErrAlreadyStarted if called more than once.
func (s *Sentinel) Start() error {
	if !s.started.CompareAndSwap(false, true) {
		return ErrAlreadyStarted
	}

	if s.config == nil {
		s.config = newDefaultSentinelConfig()
	}
	s.validateAndNormalizeConfig()

	// Sentinel is a background long-running process, thus, it uses its own context.
	s.rootProcessCtx, s.rootProcessCtxCancel = context.WithCancel(context.Background())
	s.taskQueue = make(chan probeTask, s.config.MaxConcurrentChecks*4) // buffered to enforce backpressure on producers
	s.taskQueueDone = make(chan struct{})                              // signals when taskQueue is closed

	// Logging is optional and can be disabled via config (by setting logger to nil).
	if s.config.Logger != nil {
		s.config.Logger.
			InfoContext(s.rootProcessCtx, "health.sentinel: starting sentinel",
				slog.Int64("num_workers", s.config.MaxConcurrentChecks),
				slog.Int("num_monitors", len(s.monitors)),
			)
		defer func() {
			s.config.Logger.
				Debug("health.sentinel: sentinel received stop signal, closing down root process")
		}()
	}

	// Worker pool to process tasks. Pool size limits concurrency to MaxConcurrentChecks.
	for i := int64(0); i < s.config.MaxConcurrentChecks; i++ {
		go func() {
			for task := range s.taskQueue {
				s.processProbe(task)
			}
		}()
	}

	cycleTicker := time.NewTicker(s.config.CheckInterval)
	defer cycleTicker.Stop()
	for {
		select {
		case <-s.rootProcessCtx.Done():
			return nil
		default:
		}
		s.performProbes()
		<-cycleTicker.C // wait for next tick
	}
}

// ErrNotStarted is returned when GracefulStop is called on a Sentinel that has not been started.
var ErrNotStarted = errors.New("sentinel not started")

// GracefulStop stops the Sentinel's health checking process gracefully, allowing in-flight probes to
// complete within the configured stop timeout.
//
// GracefulStop returns ErrNotStarted if Start was never called.
// Calling GracefulStop multiple times is safe; subsequent calls return nil immediately.
//
// If the stop timeout is exceeded, GracefulStop returns context.DeadlineExceeded but still
// ensures all goroutines are released (channels are closed).
func (s *Sentinel) GracefulStop() error {
	if !s.started.Load() {
		return ErrNotStarted
	}
	if !s.stopped.CompareAndSwap(false, true) {
		return nil // already stopped
	}

	timedOut := false
	if s.config.Logger != nil {
		s.config.Logger.
			Info("health.sentinel: initiating graceful sentinel shutdown")
		defer func() {
			if timedOut {
				s.config.Logger.Warn("health.sentinel: graceful shutdown timed out, forcing shutdown")
				return
			}
			s.config.Logger.Info("health.sentinel: gracefully shutdown sentinel")
		}()
	}

	// Signal shutdown: cancel context to stop main loop from starting new cycles.
	s.rootProcessCtxCancel()

	// Signal senders to stop sending tasks (must happen before closing taskQueue).
	close(s.taskQueueDone)

	// Wait for in-flight check processes to complete (with timeout).
	waitOk := syncx.WaitWithTimeout(&s.inFlightProcWg, s.config.StopTimeout)

	// Always close taskQueue to release worker goroutines (even on timeout).
	close(s.taskQueue)

	if !waitOk {
		return context.DeadlineExceeded
	}
	return nil
}

func (s *Sentinel) performProbes() {
	// Track this cycle in the WaitGroup so GracefulStop waits for it to complete.
	s.inFlightProcWg.Add(1)
	defer s.inFlightProcWg.Done()

	// This context is intentionally not linked to rootProcessCtx to allow in-flight probes
	// to complete even if shutdown is initiated.
	//
	// A context per cycle is created to enforce the overall cycle timeout.
	// Individual probes have their own timeout derived from this context.
	//
	// This design ensures that each check cycle is bounded in time, preventing long-running
	// probes from blocking later cycles.
	cycle := &probeCycle{
		wg: sync.WaitGroup{}, // tracks individual probe completions within this cycle
	}
	var cancelFunc context.CancelFunc
	cycle.ctx, cancelFunc = context.WithTimeout(context.Background(), s.config.CheckInterval)

	// logging
	if s.config.Logger != nil {
		s.config.Logger.DebugContext(cycle.ctx, "health.sentinel: starting probe cycle",
			slog.Int("total_monitors", len(s.monitors)),
		)
	}

	// tracing
	var span trace.Span
	cycle.ctx, span = s.config.Tracer.Start(cycle.ctx, "health.sentinel.probe_cycle",
		trace.WithSpanKind(trace.SpanKindConsumer), // as this is a worker consuming tasks, it must use Consumer kind
	)
	cycle.cycleSpan = span

	// Queue tasks for all monitors.
	// cycle.wg is incremented inside sendTask only for tasks that are actually queued.
	for name := range s.monitors {
		s.sendTask(probeTask{
			cycle:       cycle,
			monitorName: name,
		})
	}

	// Wait for all probes in this cycle to complete asynchronously.
	// Use a bounded wait to prevent goroutine leaks on shutdown.
	go func() {
		defer cancelFunc()
		defer span.End()

		// Wait with timeout to prevent indefinite blocking.
		// Use CheckInterval as the maximum wait time for this cycle.
		done := make(chan struct{})
		go func() {
			cycle.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All probes completed normally.
		case <-time.After(s.config.CheckInterval + s.config.CheckTimeout):
			// Timeout waiting for probes - cycle context cancellation will handle cleanup.
			if s.config.Logger != nil {
				s.config.Logger.WarnContext(cycle.ctx, "health.sentinel: probe cycle wait timed out")
			}
		}

		if s.config.Logger != nil {
			s.config.Logger.DebugContext(cycle.ctx, "health.sentinel: probe cycle finished")
		}
	}()
}

// sendTask safely sends a task to the worker pool.
// It increments WaitGroups only when the task is successfully queued.
// Returns false if the task could not be queued (shutdown in progress).
func (s *Sentinel) sendTask(task probeTask) bool {
	if s.taskQueue == nil {
		return false // Sentinel is not started yet.
	}

	// Increment WaitGroups before attempting to send.
	// If send fails, we decrement them.
	s.inFlightProcWg.Add(1)
	task.cycle.wg.Add(1)

	// Use select to prevent panic on send to closed channel.
	select {
	case <-s.taskQueueDone:
		// Shutdown in progress, revert WaitGroup increments.
		s.inFlightProcWg.Done()
		task.cycle.wg.Done()
		return false
	case s.taskQueue <- task:
		return true
	}
}

func (s *Sentinel) processProbe(task probeTask) {
	defer s.inFlightProcWg.Done()
	defer task.cycle.wg.Done()

	ctx, span := s.config.Tracer.Start(task.cycle.ctx, fmt.Sprintf("health.sentinel.monitor.%s", task.monitorName),
		trace.WithSpanKind(trace.SpanKindClient), // as this is an operation inside a probe cycle, it must use Client kind
	)
	defer span.End()

	var err error
	// logging - only log errors to reduce noise
	defer func() {
		if s.config.Logger == nil || err == nil {
			return
		}
		s.config.Logger.ErrorContext(ctx, "health.sentinel: probe failed", slog.String("monitor", task.monitorName),
			logx.Error(err),
		)
	}()
	// tracing
	defer func() {
		if err == nil {
			span.SetStatus(codes.Ok, "probe succeeded")
			return
		}
		span.SetStatus(codes.Error, err.Error())
	}()

	// Use RLock to safely access the monitors map (though it shouldn't change after Start).
	s.mu.RLock()
	monitor, exists := s.monitors[task.monitorName]
	s.mu.RUnlock()

	if !exists {
		err = fmt.Errorf("health.sentinel: monitor '%s' not found", task.monitorName)
		return
	}

	timedCtx, cancelFunc := context.WithTimeout(ctx, s.config.CheckTimeout)
	defer cancelFunc()

	// runOnRecovery captures panics from user-defined Monitor implementations.
	// Panics are converted to errors and handled gracefully with logging/tracing.
	err = runOnRecovery(func() error {
		return monitor.Check(timedCtx)
	})
}

// - Config and Options

// SentinelConfig holds configuration for a Sentinel.
type SentinelConfig struct {
	CheckInterval       time.Duration // interval between probe cycles (default: 60s)
	MaxConcurrentChecks int64         // max concurrent probes (default: max(runtime.NumCPU(), 4))
	CheckTimeout        time.Duration // timeout for each probe (default: 10s)
	StopTimeout         time.Duration // timeout for graceful shutdown (default: 60s)
	Logger              *slog.Logger  // optional logger (nil to disable logging)
	Tracer              trace.Tracer  // optional tracer for spans (nil to disable tracing)
}

func newDefaultSentinelConfig() *SentinelConfig {
	return &SentinelConfig{
		CheckInterval:       60 * time.Second,
		MaxConcurrentChecks: max(int64(runtime.NumCPU()), 4), // at least 4; health checks are I/O-bound
		CheckTimeout:        10 * time.Second,
		StopTimeout:         60 * time.Second,
		Logger:              slog.Default(),
	}
}

// validateAndNormalizeConfig ensures config values are valid, applying sensible defaults for invalid values.
func (s *Sentinel) validateAndNormalizeConfig() {
	cfg := s.config

	// Ensure positive worker count.
	if cfg.MaxConcurrentChecks < 1 {
		cfg.MaxConcurrentChecks = 1
	}

	// Ensure positive intervals.
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 30 * time.Second
	}
	if cfg.CheckTimeout <= 0 {
		cfg.CheckTimeout = 10 * time.Second
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = 30 * time.Second
	}

	// Warn if CheckTimeout exceeds CheckInterval (probes may overlap).
	if cfg.CheckTimeout > cfg.CheckInterval && cfg.Logger != nil {
		cfg.Logger.Warn("health.sentinel: check_timeout exceeds check_interval, probes may overlap",
			slog.String("check_timeout", cfg.CheckTimeout.String()), slog.String("check_interval", cfg.CheckInterval.String()))
	}
}

// SentinelOption defines a functional option for configuring the Sentinel.
type SentinelOption func(*SentinelConfig)

// WithCheckInterval sets the interval between health probes.
func WithCheckInterval(interval time.Duration) SentinelOption {
	return func(cfg *SentinelConfig) {
		cfg.CheckInterval = interval
	}
}

// WithMaxConcurrentChecks sets the maximum number of concurrent health probes.
func WithMaxConcurrentChecks(max int64) SentinelOption {
	return func(cfg *SentinelConfig) {
		cfg.MaxConcurrentChecks = max
	}
}

// WithCheckTimeout sets the timeout for each health check.
func WithCheckTimeout(timeout time.Duration) SentinelOption {
	return func(cfg *SentinelConfig) {
		cfg.CheckTimeout = timeout
	}
}

// WithStopTimeout sets the timeout for graceful shutdown.
func WithStopTimeout(timeout time.Duration) SentinelOption {
	return func(cfg *SentinelConfig) {
		cfg.StopTimeout = timeout
	}
}

// WithLogger sets the logger for the Sentinel.
func WithLogger(logger *slog.Logger) SentinelOption {
	return func(cfg *SentinelConfig) {
		cfg.Logger = logger
	}
}

// WithTracer sets the tracer for the Sentinel.
func WithTracer(tracer trace.Tracer) SentinelOption {
	return func(cfg *SentinelConfig) {
		cfg.Tracer = tracer
	}
}

// - Utils

type probeCycle struct {
	ctx         context.Context
	wg          sync.WaitGroup
	mu          sync.Mutex
	cycleSpan   trace.Span
	worstStatus string
}

type probeTask struct {
	cycle       *probeCycle
	monitorName string
}

// runOnRecovery is a helper to run a function and recover from panics, returning them as errors.
func runOnRecovery(fn func() error) (err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		} else if e, ok := r.(error); ok {
			err = e
			return
		}

		err = fmt.Errorf("panic recovered: %v", r)
	}()
	err = fn()
	return
}
