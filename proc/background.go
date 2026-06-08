package proc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bosonicalcom/bedrock-go/syncx"
)

type RunFunc func(ctx context.Context) error
type ShutdownFunc func(ctx context.Context) error
type OnShutdownFunc func(ctx context.Context)

// BackgroundProcess is a kind of process that runs for an indefinite set of time, blocking the I/O until
// some event triggers its termination.
type BackgroundProcess struct {
	Name         string
	RunFunc      RunFunc
	ShutdownFunc ShutdownFunc
}

// A BackgroundFactory is a factory that produces [BackgroundProcess] instances.
type BackgroundFactory interface {
	BackgroundProcess() BackgroundProcess
}

// RunBackground runs a set of [BackgroundProcess] instances. The parent process will block I/O for the caller's
// goroutine. It can only be terminated when one of these signals are triggered: syscall.SIGTERM,
// os.Interrupt.
//
// If atomic run is set, this routine will terminate other child processes if one of [BackgroundProcess] items
// returns an error during a running process.
//
// Moreover, a shutdown timeout is enforced to avoid goroutine deadlocks. The default value is 60 seconds.
func RunBackground(ctx context.Context, procs []BackgroundProcess, opts ...RunBackgroundOption) error {
	options := &runBackgroundOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if len(procs) == 0 {
		return errors.New("bedrock.proc: no processes to run")
	}

	if options.shutdownTimeout == 0 {
		options.shutdownTimeout = time.Minute
	}

	if options.terminationChan == nil {
		options.terminationChan = make(chan os.Signal, 1)
	}
	defer func() {
		if options.terminationChan != nil {
			close(options.terminationChan)
			options.terminationChan = nil
		}
	}()
	signal.Notify(options.terminationChan, syscall.SIGTERM, os.Interrupt)
	errsMu := &sync.Mutex{}
	isShutdown := false
	errs := make([]error, 0, len(procs))
	for i := range procs {
		if procs[i].RunFunc == nil {
			continue
		}
		go func(idx int) {
			safeLog(options.logger, slog.LevelInfo, "starting background process",
				slog.String("name", procs[idx].Name),
			)
			errRun := procs[idx].RunFunc(ctx) // blocks i/o
			if errRun == nil {
				return
			}
			errsMu.Lock()
			errs = append(errs, fmt.Errorf("bedrock.proc: failed to run background process %s: %w", procs[idx].Name,
				errRun))
			if options.isAtomicRun && !isShutdown {
				isShutdown = true
				if options.terminationChan != nil {
					options.terminationChan <- syscall.SIGTERM
				}
			}
			errsMu.Unlock()
		}(i)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-options.terminationChan:
	}
	signal.Stop(options.terminationChan)
	shutdownCtx, cancel := context.WithTimeout(ctx, options.shutdownTimeout)
	defer cancel()
	wg := sync.WaitGroup{}
	for i := range procs {
		if procs[i].ShutdownFunc == nil {
			continue
		}
		wg.Go(func() {
			safeLog(options.logger, slog.LevelInfo, "shutting down background process",
				slog.String("name", procs[i].Name),
			)
			if errShutdown := procs[i].ShutdownFunc(shutdownCtx); errShutdown != nil {
				errsMu.Lock()
				errs = append(errs, fmt.Errorf("bedrock.proc: failed to stop background process %s: %w", procs[i].Name,
					errShutdown))
				errsMu.Unlock()
			}
			safeLog(options.logger, slog.LevelInfo, "background process shut down",
				slog.String("name", procs[i].Name),
			)
		})
	}
	if options.onShutdown != nil {
		options.onShutdown(shutdownCtx)
	}
	syncx.WaitWithTimeout(&wg, options.shutdownTimeout)
	return errors.Join(errs...)
}

// -- Option --

type runBackgroundOptions struct {
	isAtomicRun     bool
	shutdownTimeout time.Duration
	onShutdown      OnShutdownFunc
	terminationChan chan os.Signal
	logger          *slog.Logger
}

// RunBackgroundOption is an options routine to configure a [RunBackground] operation.
type RunBackgroundOption func(*runBackgroundOptions)

// WithAtomicRun indicates a [RunBackground] process to shut down if one of its child processes
// got a failure during a running process.
func WithAtomicRun() RunBackgroundOption {
	return func(o *runBackgroundOptions) {
		o.isAtomicRun = true
	}
}

// WithShutdownTimeout sets the timeout for a shutdown operation in a [RunBackground] process.
func WithShutdownTimeout(timeout time.Duration) RunBackgroundOption {
	return func(o *runBackgroundOptions) {
		o.shutdownTimeout = timeout
	}
}

// WithOnShutdown appends a hook routine ([OnShutdownFunc]) to be called once a shutdown signal was received
// in a [RunBackground] process.
func WithOnShutdown(onShutdown OnShutdownFunc) RunBackgroundOption {
	return func(o *runBackgroundOptions) {
		o.onShutdown = onShutdown
	}
}

// WithTerminationChan sets the channel for [RunBackground] process termination.
//
// Channel must be buffered with a factor of 1.
//
// Mostly used for testing purposes.
func WithTerminationChan(terminationChan chan os.Signal) RunBackgroundOption {
	return func(o *runBackgroundOptions) {
		o.terminationChan = terminationChan
	}
}

// WithLogger sets the logger for the [RunBackground] process.
func WithLogger(logger *slog.Logger) RunBackgroundOption {
	return func(options *runBackgroundOptions) {
		if logger == nil {
			return
		}
		options.logger = logger
	}
}

// - util

func safeLog(logger *slog.Logger, lvl slog.Level, msg string, attr ...any) {
	if logger == nil {
		return
	}
	logger.Log(context.Background(), lvl, msg, attr...)
}
