package main

import (
	"errors"
	"testing"
)

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bare_11_char", "dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"watch_url", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"watch_extra_params", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42", "dQw4w9WgXcQ"},
		{"youtu_be", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"shorts", "https://www.youtube.com/shorts/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"embed", "https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"no_scheme_watch", "youtube.com/watch?v=dQw4w9WgXcQ", ""},
		{"invalid_url", "not a url at all", ""},
		{"empty", "", ""},
		{"too_short", "abc", ""},
		{"with_spaces", "  dQw4w9WgXcQ  ", "dQw4w9WgXcQ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVideoID(tt.input)
			if got != tt.want {
				t.Errorf("extractVideoID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTimedTextXML_ClassicFormat(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<transcript>
  <text start="0" dur="2.5">Hello world</text>
  <text start="2.5" dur="3">This is a test</text>
  <text start="5.5" dur="2">Goodbye</text>
</transcript>`

	result, err := parseTimedTextXML([]byte(xml))
	if err != nil {
		t.Fatalf("parseTimedTextXML failed: %v", err)
	}
	if result != "Hello world This is a test Goodbye" {
		t.Errorf("got %q", result)
	}
}

func TestParseTimedTextXML_Srv3Format(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<timedtext>
<body>
  <p t="0" d="2500"><s>Hello</s><s> world</s></p>
  <p t="2500" d="3000"><s>Second line</s></p>
</body>
</timedtext>`

	result, err := parseTimedTextXML([]byte(xml))
	if err != nil {
		t.Fatalf("parseTimedTextXML failed: %v", err)
	}
	if result != "Hello world Second line" {
		t.Errorf("got %q", result)
	}
}

func TestParseTimedTextXML_HTMLEntities(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<transcript>
  <text start="0" dur="2">It&apos;s a &quot;test&quot; &amp; more</text>
</transcript>`

	result, err := parseTimedTextXML([]byte(xml))
	if err != nil {
		t.Fatalf("parseTimedTextXML failed: %v", err)
	}
	if result != `It's a "test" & more` {
		t.Errorf("got %q", result)
	}
}

func TestParseTimedTextXML_Empty(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?><transcript></transcript>`
	result, err := parseTimedTextXML([]byte(xml))
	if err != nil {
		t.Fatalf("parseTimedTextXML failed: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestParseTimedTextXML_InvalidXML(t *testing.T) {
	_, err := parseTimedTextXML([]byte("not xml at all"))
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}

func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", `{"key":"value"}`, `{"key":"value"}`},
		{"nested", `{"a":{"b":"c"},"d":[1,2]}`, `{"a":{"b":"c"},"d":[1,2]}`},
		{"with_trailing", `{"key":"value"}extra stuff`, `{"key":"value"}`},
		{"empty_object", `{}`, `{}`},
		{"no_opening_brace", `no json here`, ""},
		{"empty_string", "", ""},
		{"string_with_braces", `{"msg":"hello {world}"}`, `{"msg":"hello {world}"}`},
		{"escaped_quotes", `{"msg":"say \"hi\""}`, `{"msg":"say \"hi\""}`},
		{"unmatched_open", `{"key":"value"`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONObject(tt.input)
			if got != tt.want {
				t.Errorf("extractJSONObject(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractCaptionURL_EnglishTrack(t *testing.T) {
	resp := map[string]any{
		"playabilityStatus": map[string]any{
			"status": "OK",
		},
		"captions": map[string]any{
			"playerCaptionsTracklistRenderer": map[string]any{
				"captionTracks": []any{
					map[string]any{
						"baseUrl":      "https://example.com/captions/fr",
						"languageCode": "fr",
					},
					map[string]any{
						"baseUrl":      "https://example.com/captions/en",
						"languageCode": "en",
					},
				},
			},
		},
	}
	url, err := extractCaptionURL("test-id", resp)
	if err != nil {
		t.Fatalf("extractCaptionURL failed: %v", err)
	}
	if url != "https://example.com/captions/en" {
		t.Errorf("got %q, want English track URL", url)
	}
}

func TestExtractCaptionURL_FallbackToFirst(t *testing.T) {
	resp := map[string]any{
		"playabilityStatus": map[string]any{"status": "OK"},
		"captions": map[string]any{
			"playerCaptionsTracklistRenderer": map[string]any{
				"captionTracks": []any{
					map[string]any{
						"baseUrl":      "https://example.com/captions/ja",
						"languageCode": "ja",
					},
				},
			},
		},
	}
	url, err := extractCaptionURL("test-id", resp)
	if err != nil {
		t.Fatalf("extractCaptionURL failed: %v", err)
	}
	if url != "https://example.com/captions/ja" {
		t.Errorf("got %q, want first track URL", url)
	}
}

func TestExtractCaptionURL_NoCaptions(t *testing.T) {
	resp := map[string]any{
		"playabilityStatus": map[string]any{"status": "OK"},
	}
	_, err := extractCaptionURL("test-id", resp)
	if !errors.Is(err, errNoCaptions) {
		t.Errorf("expected errNoCaptions, got %v", err)
	}
}

func TestExtractCaptionURL_Blocked(t *testing.T) {
	resp := map[string]any{
		"playabilityStatus": map[string]any{
			"status": "LOGIN_REQUIRED",
			"reason": "Sign in to confirm your age",
		},
	}
	_, err := extractCaptionURL("test-id", resp)
	if !errors.Is(err, errBlocked) {
		t.Errorf("expected errBlocked, got %v", err)
	}
}

func TestExtractCaptionURL_EmptyTracks(t *testing.T) {
	resp := map[string]any{
		"playabilityStatus": map[string]any{"status": "OK"},
		"captions": map[string]any{
			"playerCaptionsTracklistRenderer": map[string]any{
				"captionTracks": []any{},
			},
		},
	}
	_, err := extractCaptionURL("test-id", resp)
	if !errors.Is(err, errNoCaptions) {
		t.Errorf("expected errNoCaptions, got %v", err)
	}
}
