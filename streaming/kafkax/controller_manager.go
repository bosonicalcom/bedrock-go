package kafkax

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bosonicalcom/bedrock-go/proc"
	"github.com/bosonicalcom/bedrock-go/syncx"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
)

type consumerEntry struct {
	topic               string
	group               string // nilable
	enableSeqProcessing bool
	offset              kgo.Offset
	client              *kgo.Client
	handler             Handler
}

type consumerJob struct {
	entryIndex int
	partition  kgo.FetchTopicPartition
}

// ControllerManager is a system component acting as a coordinator for Kafka partition records fetching and processing.
//
// By default, the manager will process partition records in "concurrent processing" (aka. high throughput) mode.
//
// Concurrent processing trades retry amplification for throughput.
// Records within a partition batch are dispatched to a bounded worker
// pool — no ordering guarantees within the batch.
//
// Commit is all-or-nothing: if any record in the batch fails, no
// offsets are advanced. The entire batch is re-delivered on the next
// poll cycle. Handlers MUST be idempotent or guarded by a dedup
// check (e.g., message_id lookup) to avoid duplicate side effects
// on redelivery.
type ControllerManager struct {
	// context solely used to stop the poll cycle (for-select statement)
	pollCycleCtx       context.Context
	pollCycleCtxCancel context.CancelFunc
	// context passed to in-flight processes
	rootProcCtx       context.Context
	rootProcCtxCancel context.CancelFunc

	partitionJobQueue     chan consumerJob
	partitionInFlightProc sync.WaitGroup
	isTerminated          atomic.Bool

	config    *ControllerManagerConfig
	consumers []consumerEntry
}

type ControllerManagerConfig struct {
	SeedBrokerAddrs       []string
	AutoTopicCreation     bool
	MaxWorkers            int
	PartitionJobQueueSize int
	HandlerTimeout        time.Duration
	Controllers           []Controller
	Interceptors          []ConsumerInterceptor // global
	Logger                *slog.Logger
	LogLevel              slog.Level
}

type ControllerManagerOption func(config *ControllerManagerConfig)

var (
	_ ConsumerRegistrar      = (*ControllerManager)(nil)
	_ proc.BackgroundFactory = (*ControllerManager)(nil)
)

// NewControllerManager allocates a new instance of a [ControllerManager].
func NewControllerManager(seeds []string, opts ...ControllerManagerOption) *ControllerManager {
	config := newDefaultControllerManagerConfig()
	for _, opt := range opts {
		opt(config)
	}
	config.SeedBrokerAddrs = seeds
	return NewControllerManagerWithConfig(config)
}

// NewControllerManagerWithConfig allocates a new instance of a [ControllerManager].
func NewControllerManagerWithConfig(config *ControllerManagerConfig) *ControllerManager {
	if config == nil {
		panic(fmt.Errorf("bedrock.kafkax: config cannot be nil"))
	}
	return &ControllerManager{
		partitionJobQueue:     make(chan consumerJob, config.PartitionJobQueueSize),
		partitionInFlightProc: sync.WaitGroup{},
		isTerminated:          atomic.Bool{},
		config:                config,
		consumers:             make([]consumerEntry, 0),
	}
}

func newDefaultControllerManagerConfig() *ControllerManagerConfig {
	return &ControllerManagerConfig{
		SeedBrokerAddrs:       nil,
		AutoTopicCreation:     false,
		MaxWorkers:            10,
		PartitionJobQueueSize: 100,
		HandlerTimeout:        time.Second * 30,
		Controllers:           make([]Controller, 0),
		Interceptors:          make([]ConsumerInterceptor, 0),
	}
}

func (c *ControllerManager) BackgroundProcess() proc.BackgroundProcess {
	return proc.BackgroundProcess{
		Name: "kafka.controller-manager",
		RunFunc: func(ctx context.Context) error {
			err := c.Start()
			if errors.Is(err, kgo.ErrClientClosed) {
				return nil
			}
			return err
		},
		ShutdownFunc: func(ctx context.Context) error {
			return c.Terminate(ctx)
		},
	}
}

func (c *ControllerManager) Register(topic string, handler Handler, opts ...ConsumerRegistrarOption) {
	options := newDefaultConsumerRegistrarOptions()
	for _, opt := range opts {
		opt(options)
	}

	if c.consumers == nil {
		c.consumers = make([]consumerEntry, 0, 1)
	}
	c.consumers = append(c.consumers, consumerEntry{
		topic:               topic,
		handler:             ConsumerInterceptorChain(handler, append(c.config.Interceptors, options.interceptors...)...), // global outermost, then consumer
		enableSeqProcessing: options.enableSeqProcessing,
		group:               options.groupName,
		offset:              options.offset,
	})
}

func (c *ControllerManager) Start() error {
	if c.config == nil {
		c.config = newDefaultControllerManagerConfig()
	}
	if c.pollCycleCtx == nil {
		c.pollCycleCtx, c.pollCycleCtxCancel = context.WithCancel(context.Background())
	}
	if c.rootProcCtx == nil {
		c.rootProcCtx, c.rootProcCtxCancel = context.WithCancel(context.Background())
	}
	if c.partitionJobQueue == nil {
		c.partitionJobQueue = make(chan consumerJob, c.config.PartitionJobQueueSize)
	}

	if err := c.setupConsumerClients(); err != nil {
		return err
	}

	// spawn partition workers
	for i := 0; i < c.config.MaxWorkers; i++ {
		go func() {
			for {
				select {
				case job, ok := <-c.partitionJobQueue:
					if !ok {
						return
					}
					c.processPartition(job.entryIndex, job.partition)
				case <-c.rootProcCtx.Done():
					return
				}
			}
		}()
	}

	// start poll loops — one goroutine per consumer, each blocks on PollFetches.
	var pollWg sync.WaitGroup
	errCh := make(chan error, len(c.consumers))

	for i := range c.consumers {
		pollWg.Add(1)
		go func(idx int) {
			defer pollWg.Done()
			for {
				select {
				case <-c.pollCycleCtx.Done():
					return
				default:
				}

				if err := c.consume(idx); err != nil {
					errCh <- err
					return
				}
			}
		}(i)
	}

	// Wait for either all poll loops to exit or the first fatal error.
	doneCh := make(chan struct{})
	go func() {
		pollWg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		return nil
	case err := <-errCh:
		c.pollCycleCtxCancel() // stop remaining poll loops
		pollWg.Wait()          // drain before returning
		return err
	}
}

func (c *ControllerManager) Terminate(ctx context.Context) error {
	if !c.isTerminated.CompareAndSwap(false, true) {
		return nil
	}
	c.pollCycleCtxCancel()                               // stop poll cycle
	syncx.WaitWithContext(ctx, &c.partitionInFlightProc) // enforce ctx timeout, avoid deadlocks
	for i := range c.consumers {
		if c.consumers[i].client != nil {
			c.consumers[i].client.Close()
		}
	}
	c.rootProcCtxCancel() // this forces killing of child tasks. does nothing in happy-path.
	return nil
}

func (c *ControllerManager) setupConsumerClients() error {
	for _, ctrl := range c.config.Controllers {
		ctrl.RegisterConsumers(c)
	}

	baseClientOpts := []kgo.Opt{
		kgo.WithContext(c.rootProcCtx),
		kgo.SeedBrokers(c.config.SeedBrokerAddrs...),
		kgo.AutoCommitMarks(),
	}
	if c.config.AutoTopicCreation {
		baseClientOpts = append(baseClientOpts, kgo.AllowAutoTopicCreation())
	}
	if c.config.Logger != nil {
		baseClientOpts = append(baseClientOpts, kgo.WithLogger(Logger(c.config.LogLevel, c.config.Logger)))
	}
	var err error
	lastSuccessIdx := -1
	defer func() {
		if err == nil || lastSuccessIdx == -1 {
			return
		}

		for i := 0; i < lastSuccessIdx+1; i++ {
			c.consumers[i].client.Close()
		}
	}()
	for i := range c.consumers {
		// create a copy of base opts
		clientOpts := append(baseClientOpts, kgo.ConsumeTopics(c.consumers[i].topic))
		if c.consumers[i].group != "" {
			clientOpts = append(clientOpts, kgo.ConsumerGroup(c.consumers[i].group))
		}
		if c.consumers[i].offset != kgo.NewOffset().AtStart() { // diff than default
			clientOpts = append(clientOpts, kgo.ConsumeResetOffset(c.consumers[i].offset))
		}
		c.consumers[i].client, err = kgo.NewClient(clientOpts...)
		if err != nil {
			lastSuccessIdx = i - 1
			return err
		}
	}
	return nil
}

// returns fatal errors only
func (c *ControllerManager) consume(idx int) error {
	// client auto commits by default (unless changed)
	// thus, once this routine receives the message, it is required to handle the record gracefully.
	// if processing fails, rely on mechanisms like dlq or retries.
	//
	// relying on offset commits is not reliable as kafka storage is append-log, serializable and thus,
	// marking a tail of a fetched set of messages will mark the rest (from the head) as acknowledged.
	fetches := c.consumers[idx].client.PollFetches(c.rootProcCtx)
	if errs := fetches.Errors(); len(errs) > 0 {
		for _, err := range errs {
			if retryable := kerr.IsRetriable(err.Err); !retryable {
				return err.Err // fatal error, stop cycling
			}
		}
	}

	if fetches.Empty() {
		return nil
	}

	fetches.EachPartition(func(partition kgo.FetchTopicPartition) {
		if c.isTerminated.Load() { // cannot schedule more tasks due a closed channel
			return
		}
		c.partitionInFlightProc.Add(1)
		// using a bounded worker pool ensures that we don't overload the system with too many concurrent tasks.
		//
		// partitions in production-ready kafka can be within the order of thousands.
		c.partitionJobQueue <- consumerJob{
			entryIndex: idx,
			partition:  partition,
		}
	})
	c.partitionInFlightProc.Wait() // wait for all partitions to finish processing
	return nil
}

func (c *ControllerManager) processPartition(idx int, partition kgo.FetchTopicPartition) {
	defer c.partitionInFlightProc.Done()
	if c.consumers[idx].enableSeqProcessing {
		// Sequential processing guarantees per-partition ordering.
		// Records are processed and marked one at a time — if record N
		// fails, processing stops and no further records in the batch
		// are marked. The next poll cycle retries from the last committed
		// offset (which may include already-processed records if the
		// auto-commit interval hadn't flushed yet).
		//
		// Handlers should still be idempotent to tolerate redelivery
		// after crash between mark and commit.
		//
		// WARNING: sequential processing is unsuitable for handlers with
		// high latency (e.g., LLM inference, external API calls). A
		// partition with 100 pending records at 10s each takes ~16 minutes
		// to drain. During this time, consumer lag grows, downstream
		// timeouts may cascade, and Kafka may trigger a rebalance if
		// processing exceeds session.timeout.ms (default 45s in franz-go).
		// For slow handlers, use concurrent mode with idempotent/dedup
		// guards instead.
		partition.EachRecord(func(record *kgo.Record) {
			ctx, cancel := context.WithTimeout(c.rootProcCtx, c.config.HandlerTimeout)
			defer cancel()
			var err error
			defer func() {
				if err != nil {
					return // ignoring error, interceptors already handle it
				}
				c.consumers[idx].client.MarkCommitRecords(record)
			}()
			err = c.consumers[idx].handler(ctx, record)
		})
		return
	}

	// concurrent processing (high throughput).
	// Concurrent processing trades retry amplification for throughput.
	// Records within a partition batch are dispatched to a bounded worker
	// pool — no ordering guarantees within the batch.
	//
	// Commit is all-or-nothing: if any record in the batch fails, no
	// offsets are advanced. The entire batch is redelivered on the next
	// poll cycle. Handlers MUST be idempotent or guarded by a dedup
	// check (e.g., message_id lookup) to avoid duplicate side effects
	// on redelivery.

	wg := sync.WaitGroup{}
	var err error
	errMu := sync.Mutex{}
	partition.EachRecord(func(record *kgo.Record) {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(c.rootProcCtx, c.config.HandlerTimeout)
			defer cancel()

			if errProc := c.consumers[idx].handler(ctx, record); errProc != nil {
				errMu.Lock()
				err = errProc
				errMu.Unlock()
				return
			}
		})
	})
	wg.Wait()

	// all-in or nothing
	if err != nil {
		return
	}
	c.consumers[idx].client.MarkCommitRecords(partition.Records...)
}

// - options

// WithControllers appends a set of [Controller] instances to a [ControllerManager].
func WithControllers(ctrls []Controller) ControllerManagerOption {
	return func(config *ControllerManagerConfig) {
		if config.Controllers == nil {
			config.Controllers = make([]Controller, 0, len(ctrls))
		}
		config.Controllers = append(config.Controllers, ctrls...)
	}
}

// WithManagerInterceptors appends a set of [ConsumerInterceptor] instances to a [ControllerManager].
//
// These will be considered as global and will be executed outermost.
func WithManagerInterceptors(interceptors ...ConsumerInterceptor) ControllerManagerOption {
	return func(config *ControllerManagerConfig) {
		if config.Interceptors == nil {
			config.Interceptors = make([]ConsumerInterceptor, 0, len(interceptors))
		}
		config.Interceptors = append(config.Interceptors, interceptors...)
	}
}

// WithManagerLogger sets the logger for each consumer client.
//
// It will log low-level errors.
func WithManagerLogger(logger *slog.Logger, lvl slog.Level) ControllerManagerOption {
	return func(config *ControllerManagerConfig) {
		config.Logger = logger
		config.LogLevel = lvl
	}
}

// EnableAutoTopicCreation allows consumers can create a topic if needed.
func EnableAutoTopicCreation() ControllerManagerOption {
	return func(config *ControllerManagerConfig) {
		config.AutoTopicCreation = true
	}
}

// WithManagerMaxWorkers sets the maximum number of workers for concurrent processing.
//
// Each worker will process a partition.
func WithManagerMaxWorkers(n int) ControllerManagerOption {
	return func(config *ControllerManagerConfig) {
		config.MaxWorkers = n
	}
}

// WithManagerQueueSize sets the maximum number of partition processing jobs that can be queued.
//
// After the queue is full, new task schedule attempts will block, resulting in an organic back pressure mechanism.
func WithManagerQueueSize(n int) ControllerManagerOption {
	return func(config *ControllerManagerConfig) {
		config.PartitionJobQueueSize = n
	}
}

// WithHandlerTimeout sets the maximum duration for a [Handler] managed by [ControllerManager].
//
// After the timeout, the handler will be considered failed.
func WithHandlerTimeout(d time.Duration) ControllerManagerOption {
	return func(config *ControllerManagerConfig) {
		config.HandlerTimeout = d
	}
}
