package profiling

// GPUQueue identifies a GPU work queue for timeline-lane placement.
type GPUQueue uint8

const (
	// QueueGraphics is the primary rendering queue.
	QueueGraphics GPUQueue = iota
	// QueueCompute is the async-compute queue.
	QueueCompute
	// QueueTransfer is the copy/upload queue.
	QueueTransfer
)

// GPUSpan is a GPU-side timed region, correlated with CPU spans by frame number.
// Tick values are raw GPU timestamp-counter readings; divide by Frequency to
// convert to seconds.
type GPUSpan struct {
	Name      string
	StartTick uint64
	EndTick   uint64
	Frequency uint64
	Queue     GPUQueue
}

// GPUQueryHandle references an in-flight GPU timestamp query.
type GPUQueryHandle uint32

// GPUTimingCollector is implemented by render backends that support timestamp
// queries. It is declared here so a future hardware backend can satisfy the
// contract without an L1 amendment.
//
// Deferred: no backend in the current software/headless render path supports
// timestamp queries, so there is no implementation in this package (INV: GPU
// timing is backend-specific and may be unavailable on a platform). GPU spans
// correlate with CPU spans via frame number, matching the L1 design.
type GPUTimingCollector interface {
	// BeginQuery opens a timestamp query on the given queue.
	BeginQuery(name string, queue GPUQueue) GPUQueryHandle
	// EndQuery closes a previously opened query.
	EndQuery(handle GPUQueryHandle)
	// ResolveQueries returns completed GPU spans, called once per frame after
	// GPU work completes.
	ResolveQueries() []GPUSpan
}
