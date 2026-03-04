package main

import (
	"testing"
)

func TestExtractJSON_PlainJSON(t *testing.T) {
	input := `{"key": "value"}`
	got := extractJSON(input)
	if got != input {
		t.Errorf("expected %q, got %q", input, got)
	}
}

func TestExtractJSON_WithWhitespace(t *testing.T) {
	input := `  {"key": "value"}  `
	want := `{"key": "value"}`
	got := extractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_CodeBlockJSON(t *testing.T) {
	input := "Here is the result:\n```json\n{\"score\": 85}\n```\nDone."
	want := `{"score": 85}`
	got := extractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_CodeBlockPlain(t *testing.T) {
	input := "Result:\n```\n{\"score\": 85}\n```"
	want := `{"score": 85}`
	got := extractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_EmbeddedBraces(t *testing.T) {
	input := "The analysis shows {\"score\": 42, \"grade\": \"B\"} as the result."
	want := `{"score": 42, "grade": "B"}`
	got := extractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "No JSON here at all"
	got := extractJSON(input)
	if got != input {
		t.Errorf("expected original text back, got %q", got)
	}
}

func TestExtractJSON_NestedJSON(t *testing.T) {
	input := `{"outer": {"inner": "value"}, "list": [1,2,3]}`
	got := extractJSON(input)
	if got != input {
		t.Errorf("expected %q, got %q", input, got)
	}
}

func TestExtractJSON_EmptyString(t *testing.T) {
	got := extractJSON("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractJSON_ComplexAnalysisOutput(t *testing.T) {
	input := `I've analyzed the website and here are my findings:

` + "```json" + `
{
  "overallScore": 72,
  "pillars": {
    "contentAuthority": {"score": 78},
    "structuralOptimization": {"score": 65}
  }
}
` + "```" + `

This shows good content authority but room for improvement in structure.`

	got := extractJSON(input)
	if got == input {
		t.Error("should have extracted JSON from code block")
	}
	if len(got) < 10 {
		t.Error("extracted JSON too short")
	}
}
