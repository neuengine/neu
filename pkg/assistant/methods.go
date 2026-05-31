//go:build editor

package assistant

// Standard agent method names (L1 §4.5). Agents declare which they support; the
// editor may also call custom methods discovered at the capability handshake.
const (
	MethodSuggestComponents = "suggest_components"
	MethodGenerateScene     = "generate_scene"
	MethodGenerateUI        = "generate_ui"
	MethodExplainEntity     = "explain_entity"
	MethodDiagnoseIssue     = "diagnose_issue"
	MethodOptimizeScene     = "optimize_scene"
	MethodGenerateCode      = "generate_code"
	MethodReviewDefinition  = "review_definition"
	MethodAutocomplete      = "autocomplete"
	MethodChat              = "chat"
)

// requiredCaps maps each standard method to the capabilities it requires
// (L1 §4.5). A method with no entry requires at least ReadTypeRegistry (the
// chat minimum). Custom methods default to ExecuteCommands (most restrictive).
var requiredCaps = map[string]Capability{
	MethodSuggestComponents: ReadTypeRegistry,
	MethodGenerateScene:     ReadTypeRegistry | WriteScenes,
	MethodGenerateUI:        ReadTypeRegistry | WriteDefinitions,
	MethodExplainEntity:     ReadScenes | ReadTypeRegistry,
	MethodDiagnoseIssue:     ReadDiagnostics,
	MethodOptimizeScene:     ReadScenes | ReadDiagnostics,
	MethodGenerateCode:      CodeGeneration,
	MethodReviewDefinition:  ReadDefinitions,
	MethodAutocomplete:      ReadTypeRegistry,
	MethodChat:              ReadTypeRegistry,
}

// RequiredCapability returns the capability a method needs. Unknown (custom)
// methods require ExecuteCommands as a conservative default.
func RequiredCapability(method string) Capability {
	if c, ok := requiredCaps[method]; ok {
		return c
	}
	return ExecuteCommands
}

// IsStandardMethod reports whether method is one of the standard methods.
func IsStandardMethod(method string) bool {
	_, ok := requiredCaps[method]
	return ok
}
