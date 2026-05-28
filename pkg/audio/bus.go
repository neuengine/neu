package audio

import "errors"

// ErrAudioBusCycle is returned when the AudioBusLayout DAG contains a cycle.
var ErrAudioBusCycle = errors.New("audio: bus graph contains a cycle")

// AudioBus represents one named mixing bus in the bus graph.
// Buses form a DAG rooted at the "Master" bus (Output == "").
type AudioBus struct {
	Name    string
	Output  string
	Effects []AudioEffectInstance
	Volume  float32
	Mute    bool
	Solo    bool
}

// AudioBusLayout is a world resource holding the full bus graph.
// ValidateDAG returns ErrAudioBusCycle if a cycle is detected.
type AudioBusLayout struct {
	Buses []AudioBus
}

// ValidateDAG checks the bus graph for cycles using DFS.
// Returns ErrAudioBusCycle if a cycle is found.
func (l *AudioBusLayout) ValidateDAG() error {
	index := make(map[string]int, len(l.Buses))
	for i, b := range l.Buses {
		index[b.Name] = i
	}
	// visited: 0=unvisited, 1=in-stack, 2=done
	visited := make([]uint8, len(l.Buses))
	var dfs func(i int) bool
	dfs = func(i int) bool {
		if visited[i] == 2 {
			return false
		}
		if visited[i] == 1 {
			return true // cycle
		}
		visited[i] = 1
		if out := l.Buses[i].Output; out != "" {
			if j, ok := index[out]; ok {
				if dfs(j) {
					return true
				}
			}
		}
		visited[i] = 2
		return false
	}
	for i := range l.Buses {
		if dfs(i) {
			return ErrAudioBusCycle
		}
	}
	return nil
}
