package render

// Stage is one of the four render-schedule stages executed per frame by
// [RenderSubApp.RunFrame] (l1-render-core §4.9). It is distinct from
// gpu.RenderPhase, which buckets draw items WITHIN the Draw stage.
//
//	Collect → Extract → Prepare → Draw
//
// Extract runs exactly once per frame and strictly before Prepare/Draw
// (l1-render-core INV-4); render passes execute only in StageDraw.
type Stage uint8

const (
	StageCollect Stage = iota // init per-frame state, advance frame counter
	StageExtract              // copy main-world data into the render world (once)
	StagePrepare              // CPU resource prep (feature-driven; T-4A04)
	StageDraw                 // execute the render graph — passes run only here
)

// String returns the stage name (diagnostics / golden traces).
func (s Stage) String() string {
	switch s {
	case StageCollect:
		return "Collect"
	case StageExtract:
		return "Extract"
	case StagePrepare:
		return "Prepare"
	case StageDraw:
		return "Draw"
	default:
		return "Stage(?)"
	}
}
