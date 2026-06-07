package language

// Token is a language-agnostic morphological token.
// Drivers populate whichever fields are meaningful for their language.
type Token struct {
	Surface  string   // word as it appears in text
	Lemma    string
	POS      string   // normalized part-of-speech label (e.g. "verb", "noun")
	Features []string // e.g. ["Tense=Pres", "Person=3", "Number=Sing"]
	                  // or   ["Case=Ine", "Number=Plur"]
}

// AnalysisResult is what a driver returns for selected text.
type AnalysisResult struct {
	Tokens []Token
	// Error is non-fatal: partial results are still rendered.
	Error string
}

// Driver is the pluggable language interface.
type Driver interface {
	// Language returns a display name used in logs and UI ("Finnish", "Spanish").
	Language() string

	// Analyze runs the deterministic tool layer on selected text.
	// Returns a result even on partial failure; sets result.Error.
	Analyze(text string) (*AnalysisResult, error)

	// SystemPrompt returns the LLM system prompt for the vetting/explanation role.
	SystemPrompt() string

	// BuildPrompt constructs the user-turn prompt sent to Ollama.
	BuildPrompt(text, context string, result *AnalysisResult) string

	// ParseResponse splits the LLM response into translation and explanation.
	ParseResponse(response string) (translation, explanation string)
}