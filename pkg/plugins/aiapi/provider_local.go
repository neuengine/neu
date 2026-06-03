//go:build editor

package aiapi

// Local OpenAI-compatible servers (ollama, llama.cpp, vLLM, LM Studio) speak the
// same chat-completions schema as OpenAI, so "local" reuses the OpenAI provider
// implementation — only the configured endpoint differs (e.g. http://localhost:
// 11434/v1). This is the L1 §2 "local LLM support via OpenAI-compatible servers"
// path: zero extra wire code, full streaming, just a different base URL.
func init() { RegisterProvider("local", newOpenAI) }
