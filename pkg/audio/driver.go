package audio

// AudioDriver is the platform-specific hardware layer.
// AudioServer sits above it and manages the bus graph and spatial computations;
// AudioDriver owns the OS audio handle and the mix goroutine.
type AudioDriver interface {
	// Init initialises the hardware device.
	Init(mixRate, channels uint32) error
	// Start starts the mix goroutine / hardware callbacks.
	Start()
	// MixRate returns the device sample rate.
	MixRate() uint32
	// Lock acquires the mix-thread mutex. Must surround buffer fills.
	Lock()
	// Unlock releases the mix-thread mutex.
	Unlock()
	// Close stops the driver and releases the OS handle.
	Close() error
}
