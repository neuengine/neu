package visualgraph

// registerBuiltins installs a representative built-in node set (L1 §4.2). The
// full taxonomy (Query, SubGraph, the complete Math/Vector/Logic libraries, and
// TypeRegistry auto-generated component/event/state nodes) is added as the graph
// plugin matures; this set exercises every execution category for the C29 gate.
func registerBuiltins(in *Interpreter) {
	// Event entry: fires the chain (no execution input; one execution output).
	in.Register("event.OnUpdate", NodeBehavior{
		Exec: func(_, _ map[string]any, _ CommandSink) (string, error) { return "exec_out", nil },
	})

	// Data: math.Add — result = a + b.
	in.Register("math.Add", NodeBehavior{
		Eval: func(_, inputs map[string]any) map[string]any {
			return map[string]any{"result": toFloat(inputs["a"]) + toFloat(inputs["b"])}
		},
	})

	// Data: logic.Greater — result = a > b.
	in.Register("logic.Greater", NodeBehavior{
		Eval: func(_, inputs map[string]any) map[string]any {
			return map[string]any{"result": toFloat(inputs["a"]) > toFloat(inputs["b"])}
		},
	})

	// Flow: Branch — follow "true" or "false" on the bool condition input.
	in.Register("flow.Branch", NodeBehavior{
		Exec: func(_, inputs map[string]any, _ CommandSink) (string, error) {
			if b, _ := inputs["cond"].(bool); b {
				return "true", nil
			}
			return "false", nil
		},
	})

	// Action: Log — emit a log command, continue.
	in.Register("action.Log", NodeBehavior{
		Exec: func(_, inputs map[string]any, sink CommandSink) (string, error) {
			sink.Emit("log", map[string]any{"msg": inputs["msg"]})
			return "exec_out", nil
		},
	})

	// Action: SetVariable — emit a set-variable command, continue.
	in.Register("action.SetVariable", NodeBehavior{
		Exec: func(props, inputs map[string]any, sink CommandSink) (string, error) {
			sink.Emit("set_var", map[string]any{"name": props["name"], "value": inputs["value"]})
			return "exec_out", nil
		},
	})
}

// toFloat coerces a numeric any to float64 (0 for nil/non-numeric).
func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}
