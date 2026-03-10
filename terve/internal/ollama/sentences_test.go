package ollama

import "testing"

func TestParseSentenceJSON_Valid(t *testing.T) {
	input := `[
		{"finnish": "Menen taloon aamulla.", "english": "I go to the house in the morning.", "target_form": "taloon"},
		{"finnish": "Talo on suuri.", "english": "The house is big.", "target_form": "Talo"}
	]`

	entries, err := parseSentenceJSON(input)
	if err != nil {
		t.Fatalf("parseSentenceJSON: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Finnish != "Menen taloon aamulla." {
		t.Errorf("finnish = %q", entries[0].Finnish)
	}
	if entries[0].TargetForm != "taloon" {
		t.Errorf("target_form = %q", entries[0].TargetForm)
	}
}

func TestParseSentenceJSON_WithFences(t *testing.T) {
	input := "```json\n" + `[
		{"finnish": "Koira juoksee puistossa.", "english": "The dog runs in the park.", "target_form": "koira"}
	]` + "\n```"

	entries, err := parseSentenceJSON(input)
	if err != nil {
		t.Fatalf("parseSentenceJSON with fences: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].English != "The dog runs in the park." {
		t.Errorf("english = %q", entries[0].English)
	}
}

func TestParseSentenceJSON_FiltersInvalid(t *testing.T) {
	input := `[
		{"finnish": "Menen taloon.", "english": "I go to the house.", "target_form": "taloon"},
		{"finnish": "Talo on suuri.", "english": "The house is big.", "target_form": "missing_word"},
		{"finnish": "", "english": "Empty finnish.", "target_form": "test"},
		{"finnish": "Has finnish.", "english": "", "target_form": "Has"}
	]`

	entries, err := parseSentenceJSON(input)
	if err != nil {
		t.Fatalf("parseSentenceJSON: %v", err)
	}
	// Only the first entry should pass: target_form in finnish, non-empty fields
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(entries))
	}
	if entries[0].TargetForm != "taloon" {
		t.Errorf("expected surviving entry to be 'taloon', got %q", entries[0].TargetForm)
	}
}

func TestParseSentenceJSON_BadJSON(t *testing.T) {
	_, err := parseSentenceJSON("not json at all")
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}
