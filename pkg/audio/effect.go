package audio

// AudioEffect is a stateless configuration resource for an audio processing stage.
// One AudioEffect instance can be shared across multiple buses; each bus holds
// its own AudioEffectInstance with independent processing state.
type AudioEffect interface {
	// CreateInstance returns a new stateful instance for one bus.
	CreateInstance() AudioEffectInstance
}

// AudioEffectInstance is the per-bus stateful processor created from an AudioEffect.
// Process is called on the mix thread; the ECS side only manages assignment.
type AudioEffectInstance interface {
	// Process applies the effect to buf in-place at the given sample rate.
	// buf is interleaved stereo (or mono) PCM, normalised [-1, 1].
	Process(buf []float32, sampleRate uint32)
}
