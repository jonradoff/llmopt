package main

import (
	"testing"
)

func TestProviderRegistry(t *testing.T) {
	expected := []string{"anthropic", "openai", "grok", "gemini"}
	for _, id := range expected {
		p := getProvider(id)
		if p == nil {
			t.Fatalf("provider %q not found in registry", id)
		}
		if p.ProviderID() != id {
			t.Errorf("provider %q: ProviderID() = %q", id, p.ProviderID())
		}
		if p.Name() == "" {
			t.Errorf("provider %q: Name() is empty", id)
		}
		if len(p.Models()) == 0 {
			t.Errorf("provider %q: Models() is empty", id)
		}
		if p.SmallModel() == "" {
			t.Errorf("provider %q: SmallModel() is empty", id)
		}
	}
}

func TestProviderRegistryUnknown(t *testing.T) {
	p := getProvider("nonexistent")
	if p != nil {
		t.Fatal("expected nil for unknown provider")
	}
}

func TestValidProviderIDs(t *testing.T) {
	ids := validProviderIDs()
	if len(ids) < 5 {
		t.Fatalf("expected at least 5 provider IDs (including youtube), got %d", len(ids))
	}

	has := map[string]bool{}
	for _, id := range ids {
		has[id] = true
	}

	for _, expected := range []string{"anthropic", "openai", "grok", "gemini", "youtube"} {
		if !has[expected] {
			t.Errorf("validProviderIDs missing %q", expected)
		}
	}
}

func TestAnthropicModels(t *testing.T) {
	p := getProvider("anthropic")
	models := p.Models()
	if len(models) == 0 {
		t.Fatal("Anthropic should have at least one model")
	}
	// First model should be the primary/recommended one
	if models[0].ID == "" || models[0].Name == "" {
		t.Error("first model has empty ID or Name")
	}
}

func TestGeminiModels(t *testing.T) {
	p := getProvider("gemini")
	models := p.Models()
	if len(models) < 2 {
		t.Fatal("Gemini should have at least 2 models")
	}
}

func TestOpenAIModels(t *testing.T) {
	p := getProvider("openai")
	models := p.Models()
	if len(models) == 0 {
		t.Fatal("OpenAI should have at least one model")
	}
}

func TestGrokModels(t *testing.T) {
	p := getProvider("grok")
	models := p.Models()
	if len(models) == 0 {
		t.Fatal("Grok should have at least one model")
	}
}

func TestBuildStreamBody(t *testing.T) {
	for _, id := range []string{"anthropic", "openai", "grok", "gemini"} {
		t.Run(id, func(t *testing.T) {
			p := getProvider(id)
			body, err := p.BuildStreamBody(p.Models()[0].ID, 1000, "test prompt", false)
			if err != nil {
				t.Fatalf("BuildStreamBody failed: %v", err)
			}
			if len(body) == 0 {
				t.Fatal("BuildStreamBody returned empty body")
			}
		})
	}
}

func TestBuildStreamBodyWithWebSearch(t *testing.T) {
	for _, id := range []string{"anthropic", "openai", "gemini"} {
		t.Run(id, func(t *testing.T) {
			p := getProvider(id)
			body, err := p.BuildStreamBody(p.Models()[0].ID, 1000, "test prompt", true)
			if err != nil {
				t.Fatalf("BuildStreamBody with web search failed: %v", err)
			}
			if len(body) == 0 {
				t.Fatal("BuildStreamBody returned empty body")
			}
		})
	}
}
