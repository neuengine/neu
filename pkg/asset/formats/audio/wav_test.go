package audio

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// encodeWAV builds a minimal valid PCM WAV byte slice.
func encodeWAV(sampleRate uint32, channels uint16, bitsPerSample uint16, samples []int16) []byte {
	pcmData := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(pcmData[i*2:], uint16(s))
	}
	dataSize := uint32(len(pcmData))
	var buf bytes.Buffer
	// RIFF header
	_, _ = buf.Write([]byte{'R', 'I', 'F', 'F'})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	_, _ = buf.Write([]byte{'W', 'A', 'V', 'E'})
	// fmt chunk
	_, _ = buf.Write([]byte{'f', 'm', 't', ' '})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16)) // chunk size
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))  // PCM
	_ = binary.Write(&buf, binary.LittleEndian, channels)
	_ = binary.Write(&buf, binary.LittleEndian, sampleRate)
	byteRate := sampleRate * uint32(channels) * uint32(bitsPerSample) / 8
	_ = binary.Write(&buf, binary.LittleEndian, byteRate)
	blockAlign := channels * bitsPerSample / 8
	_ = binary.Write(&buf, binary.LittleEndian, blockAlign)
	_ = binary.Write(&buf, binary.LittleEndian, bitsPerSample)
	// data chunk
	_, _ = buf.Write([]byte{'d', 'a', 't', 'a'})
	_ = binary.Write(&buf, binary.LittleEndian, dataSize)
	_, _ = buf.Write(pcmData)
	return buf.Bytes()
}

// TestWAVLoader_SampleRate verifies WAV decode produces correct sample rate and
// channel count (T-5T04 codec golden — WAV decode sample-rate/PCM).
func TestWAVLoader_SampleRate(t *testing.T) {
	const sampleRate = 44100
	const channels = 2
	samples := []int16{0, 1000, -1000, 32767, -32768}
	data := encodeWAV(sampleRate, channels, 16, samples)

	l := WAVLoader{}
	got, err := l.Load(bytes.NewReader(data), LoadSettings{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SampleRate != sampleRate {
		t.Errorf("SampleRate = %d, want %d", got.SampleRate, sampleRate)
	}
	if got.Channels != channels {
		t.Errorf("Channels = %d, want %d", got.Channels, channels)
	}
	if len(got.Samples) != len(samples) {
		t.Errorf("Samples len = %d, want %d", len(got.Samples), len(samples))
	}
	// Verify normalisation: 32767 → ~1.0.
	if got.Samples[3] < 0.99 || got.Samples[3] > 1.01 {
		t.Errorf("max sample = %v, want ≈1.0", got.Samples[3])
	}
	// Verify normalisation: -32768 → ~-1.0.
	if got.Samples[4] > -0.99 || got.Samples[4] < -1.01 {
		t.Errorf("min sample = %v, want ≈-1.0", got.Samples[4])
	}
}

func TestWAVLoader_Extensions(t *testing.T) {
	l := WAVLoader{}
	exts := l.Extensions()
	found := false
	for _, e := range exts {
		if e == ".wav" {
			found = true
		}
	}
	if !found {
		t.Error("WAVLoader does not report .wav extension")
	}
}

func TestWAVLoader_InvalidData(t *testing.T) {
	l := WAVLoader{}
	_, err := l.Load(bytes.NewReader([]byte("not a wav")), LoadSettings{})
	if err == nil {
		t.Error("expected error for invalid WAV data")
	}
}

func TestWAVLoader_MissingDataChunk(t *testing.T) {
	// Minimal RIFF+WAVE+fmt but no data chunk.
	var buf bytes.Buffer
	buf.Write([]byte{'R', 'I', 'F', 'F'})
	binary.Write(&buf, binary.LittleEndian, uint32(36))
	buf.Write([]byte{'W', 'A', 'V', 'E'})
	buf.Write([]byte{'f', 'm', 't', ' '})
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))     // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(2))     // channels
	binary.Write(&buf, binary.LittleEndian, uint32(44100)) // sample rate
	binary.Write(&buf, binary.LittleEndian, uint32(176400))
	binary.Write(&buf, binary.LittleEndian, uint16(4))
	binary.Write(&buf, binary.LittleEndian, uint16(16))

	l := WAVLoader{}
	_, err := l.Load(&buf, LoadSettings{})
	if err == nil {
		t.Error("expected error for WAV without data chunk")
	}
}
