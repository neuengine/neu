// Package audio provides stdlib-only audio asset loaders.
// WAV decoding is pure Go (encoding/binary); no external audio libs required (C-003).
//
// Bootstrap: l2-asset-formats-go Draft (C29 P5 gate open).
package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/neuengine/neu/pkg/asset"
	pkgaudio "github.com/neuengine/neu/pkg/audio"
)

// LoadSettings carries optional audio decoding hints.
type LoadSettings struct{}

// ErrUnsupportedWAV is returned for WAV files with unsupported codec or bit depth.
var ErrUnsupportedWAV = errors.New("wav: unsupported format (only PCM 16-bit stereo/mono supported)")

// WAVLoader decodes uncompressed PCM WAV files into an AudioSource asset.
// Supported: PCM (format tag 1), 8-bit or 16-bit, mono or stereo (INV-2).
type WAVLoader struct{}

var _ asset.AssetLoader[pkgaudio.AudioSource, LoadSettings] = WAVLoader{}

func (WAVLoader) Extensions() []string { return []string{".wav"} }

func (WAVLoader) Load(r io.Reader, _ LoadSettings) (pkgaudio.AudioSource, error) {
	return decodeWAV(r)
}

// RegisterAll registers the WAV loader with the given AssetServer.
func RegisterAll(srv *asset.AssetServer) error {
	if srv == nil {
		return errors.New("asset/formats/audio: nil AssetServer")
	}
	asset.RegisterLoader[pkgaudio.AudioSource, LoadSettings](srv, WAVLoader{})
	return nil
}

// ── WAV decoder ──────────────────────────────────────────────────────────────

type riffHeader struct {
	ChunkID   [4]byte // "RIFF"
	ChunkSize uint32
	Format    [4]byte // "WAVE"
}

type fmtChunk struct {
	AudioFormat   uint16 // 1 = PCM
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
}

func decodeWAV(r io.Reader) (pkgaudio.AudioSource, error) {
	var hdr riffHeader
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return pkgaudio.AudioSource{}, fmt.Errorf("wav: read RIFF header: %w", err)
	}
	if hdr.ChunkID != [4]byte{'R', 'I', 'F', 'F'} {
		return pkgaudio.AudioSource{}, errors.New("wav: not a RIFF file")
	}
	if hdr.Format != [4]byte{'W', 'A', 'V', 'E'} {
		return pkgaudio.AudioSource{}, errors.New("wav: not WAVE format")
	}

	var fmt_ fmtChunk
	var dataSize uint32
	var rawSamples []byte
	var fmtFound, dataFound bool

	// Read chunks until we have fmt + data.
	for {
		var id [4]byte
		var size uint32
		if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
			break
		}
		if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
			break
		}
		switch id {
		case [4]byte{'f', 'm', 't', ' '}:
			if err := binary.Read(r, binary.LittleEndian, &fmt_); err != nil {
				return pkgaudio.AudioSource{}, fmt.Errorf("wav: read fmt chunk: %w", err)
			}
			// Skip extra bytes if fmt chunk is larger than fmtChunk struct.
			extra := int(size) - binary.Size(fmt_)
			if extra > 0 {
				if _, err := io.ReadFull(r, make([]byte, extra)); err != nil {
					return pkgaudio.AudioSource{}, fmt.Errorf("wav: skip fmt extra: %w", err)
				}
			}
			fmtFound = true
		case [4]byte{'d', 'a', 't', 'a'}:
			rawSamples = make([]byte, size)
			if _, err := io.ReadFull(r, rawSamples); err != nil {
				return pkgaudio.AudioSource{}, fmt.Errorf("wav: read data: %w", err)
			}
			dataSize = size
			dataFound = true
		default:
			// Skip unknown chunks.
			_, _ = io.ReadFull(r, make([]byte, size))
		}
		if fmtFound && dataFound {
			break
		}
	}

	if !fmtFound || !dataFound {
		return pkgaudio.AudioSource{}, errors.New("wav: missing fmt or data chunk")
	}
	if fmt_.AudioFormat != 1 {
		return pkgaudio.AudioSource{}, ErrUnsupportedWAV
	}
	if fmt_.BitsPerSample != 8 && fmt_.BitsPerSample != 16 {
		return pkgaudio.AudioSource{}, ErrUnsupportedWAV
	}
	_ = dataSize

	// Convert raw bytes to normalised float32 samples.
	samples := convertPCM(rawSamples, fmt_.BitsPerSample)
	return pkgaudio.AudioSource{
		Samples:    samples,
		SampleRate: fmt_.SampleRate,
		Channels:   fmt_.NumChannels,
	}, nil
}

func convertPCM(raw []byte, bitsPerSample uint16) []float32 {
	if bitsPerSample == 16 {
		n := len(raw) / 2
		out := make([]float32, n)
		for i := range n {
			s := int16(binary.LittleEndian.Uint16(raw[i*2:]))
			out[i] = float32(s) / 32768.0
		}
		return out
	}
	// 8-bit PCM is unsigned (0–255), centre at 128.
	out := make([]float32, len(raw))
	for i, b := range raw {
		out[i] = (float32(b) - 128) / 128.0
	}
	return out
}
