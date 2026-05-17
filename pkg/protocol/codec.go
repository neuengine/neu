package protocol

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

// ErrUnknownKind is returned by Decode when the "type" field carries a Kind
// that this version of the codec does not recognize. Callers should skip and
// continue reading — forward-compat rule (l2-multi-repo-architecture-go §Key Methods).
var ErrUnknownKind = errors.New("protocol: unknown message kind")

// Encode writes msg as a single JSON object followed by a newline ('\n').
// msg must be one of the protocol message structs with its Type field set.
func Encode(w io.Writer, msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

// Decode reads the next newline-delimited message from r.
// It peeks the "type" field, then unmarshals into the matching concrete struct.
// Returns (nil, kind, ErrUnknownKind) for unrecognized kinds so the caller
// can skip and continue (forward-compat).
// Returns io.EOF at a clean stream end.
func Decode(r *bufio.Reader) (any, Kind, error) {
	line, err := r.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return nil, "", err
	}

	var peek struct {
		Type Kind `json:"type"`
	}
	if err := json.Unmarshal(line, &peek); err != nil {
		return nil, "", err
	}

	switch peek.Type {
	case KindHotReloadPrepare:
		return unmarshalInto[HotReloadPrepare](line, peek.Type)
	case KindHotReloadReady:
		return unmarshalInto[HotReloadReady](line, peek.Type)
	case KindHotReloadFailed:
		return unmarshalInto[HotReloadFailed](line, peek.Type)
	case KindShaderError:
		return unmarshalInto[ShaderError](line, peek.Type)
	case KindShaderReloaded:
		return unmarshalInto[ShaderReloaded](line, peek.Type)
	case KindReloadMetrics:
		return unmarshalInto[ReloadMetrics](line, peek.Type)
	case KindNetworkAlert:
		return unmarshalInto[NetworkAlert](line, peek.Type)
	case KindDiagnosticSnap:
		return unmarshalInto[DiagnosticSnapshot](line, peek.Type)
	default:
		return nil, peek.Type, ErrUnknownKind
	}
}

func unmarshalInto[T any](data []byte, kind Kind) (any, Kind, error) {
	var m T
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, kind, err
	}
	return m, kind, nil
}

// Scanner wraps a bufio.Reader and exposes a Next()-based iteration API
// over newline-delimited protocol messages.
type Scanner struct {
	r *bufio.Reader
}

// NewScanner returns a Scanner reading from r.
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{r: bufio.NewReader(r)}
}

// Next reads and decodes the next message.
// Returns (msg, kind, true, nil) on success.
// Returns (nil, "", false, nil) at clean EOF.
// Returns (nil, kind, false, err) on decode error.
func (s *Scanner) Next() (any, Kind, bool, error) {
	msg, kind, err := Decode(s.r)
	if errors.Is(err, io.EOF) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, kind, false, err
	}
	return msg, kind, true, nil
}
