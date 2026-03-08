package voikko

// MorphAnalysis is a single morphological interpretation of a word.
type MorphAnalysis struct {
	Lemma            string `json:"lemma"`
	WordClass        string `json:"word_class"`
	WordClassEnglish string `json:"word_class_english,omitempty"`
	Case             string `json:"case,omitempty"`
	Number    string `json:"number,omitempty"`
	Person    string `json:"person,omitempty"`
	Tense     string `json:"tense,omitempty"`
	Mood      string `json:"mood,omitempty"`
	Possessive string `json:"possessive,omitempty"`
}

// WordValidation is the result of validating a single word.
type WordValidation struct {
	Word      string         `json:"word"`
	Valid     bool           `json:"valid"`
	Analyses  []MorphAnalysis `json:"analyses"`
	Ambiguous bool           `json:"ambiguous"`
}

// TokenAnalysis is a single token from sentence validation.
type TokenAnalysis struct {
	Token    string         `json:"token"`
	Type     string         `json:"type"`
	Valid    bool           `json:"valid,omitempty"`
	Analyses []MorphAnalysis `json:"analyses,omitempty"`
}

// SentenceValidation is the result of validating a sentence.
type SentenceValidation struct {
	Sentence     string          `json:"sentence"`
	Tokens       []TokenAnalysis `json:"tokens"`
	InvalidWords []string        `json:"invalid_words"`
	AllValid     bool            `json:"all_valid"`
}

// VowelHarmony is the result of vowel harmony analysis.
type VowelHarmony struct {
	Word         string `json:"word"`
	Harmony      string `json:"harmony"`
	SuffixVowelA string `json:"suffix_vowel_a"`
	SuffixVowelO string `json:"suffix_vowel_o"`
	SuffixVowelU string `json:"suffix_vowel_u"`
}

// Suggestions is the result of a spelling suggestion request.
type Suggestions struct {
	Word        string   `json:"word"`
	Valid       bool     `json:"valid"`
	Suggestions []string `json:"suggestions"`
}

// BatchResult is the result of a batch validation.
type BatchResult struct {
	Word     string         `json:"word"`
	Valid    bool           `json:"valid"`
	Analyses []MorphAnalysis `json:"analyses"`
}
