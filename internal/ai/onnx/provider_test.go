//go:build with_onnx

package onnx

import (
	"context"
	"math"
	"os"
	"testing"
)

// requireModelFiles skips the test when the embedded model bytes are absent.
// This happens when the binary is built without running download_ai_deps.sh.
func requireModelFiles(t *testing.T) {
	t.Helper()
	if len(modelONNX) == 0 || len(tokenizerJSON) == 0 {
		t.Skip("model files not embedded; run scripts/download_ai_deps.sh then rebuild with -tags with_onnx")
	}
}

func newTestProvider(t *testing.T) *ONNXProvider {
	t.Helper()
	requireModelFiles(t)
	p, err := newONNXProvider()
	if err != nil {
		t.Fatalf("newONNXProvider: %v", err)
	}
	t.Cleanup(func() {
		if p.session != nil {
			p.session.Destroy()
		}
		if p.tok != nil {
			p.tok.Close()
		}
	})
	return p
}

// ── Embed ─────────────────────────────────────────────────────────────────────

func TestEmbed_returns384Dims(t *testing.T) {
	p := newTestProvider(t)
	vec, err := p.Embed(context.Background(), "the quick brown fox")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != embeddingDim {
		t.Errorf("want %d dims, got %d", embeddingDim, len(vec))
	}
}

func TestEmbed_vectorIsNormalised(t *testing.T) {
	p := newTestProvider(t)
	vec, err := p.Embed(context.Background(), "normalisation check")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 0.01 {
		t.Errorf("want unit vector (norm≈1), got norm=%f", norm)
	}
}

func TestEmbed_similarSentences_highCosine(t *testing.T) {
	p := newTestProvider(t)
	a, err := p.Embed(context.Background(), "the cat sat on the mat")
	if err != nil {
		t.Fatalf("Embed a: %v", err)
	}
	b, err := p.Embed(context.Background(), "a cat was sitting on a mat")
	if err != nil {
		t.Fatalf("Embed b: %v", err)
	}
	sim := cosineSim(a, b)
	if sim < 0.85 {
		t.Errorf("similar sentences: want cosine > 0.85, got %.4f", sim)
	}
}

func TestEmbed_dissimilarSentences_lowCosine(t *testing.T) {
	p := newTestProvider(t)
	a, err := p.Embed(context.Background(), "the cat sat on the mat")
	if err != nil {
		t.Fatalf("Embed a: %v", err)
	}
	b, err := p.Embed(context.Background(), "quarterly invoice payment due")
	if err != nil {
		t.Fatalf("Embed b: %v", err)
	}
	sim := cosineSim(a, b)
	if sim > 0.5 {
		t.Errorf("dissimilar sentences: want cosine < 0.5, got %.4f", sim)
	}
}

// ── ClassifyThought ───────────────────────────────────────────────────────────

func TestClassifyThought_todoText(t *testing.T) {
	p := newTestProvider(t)
	result, err := p.ClassifyThought("fix the authentication bug before the sprint ends")
	if err != nil {
		t.Fatalf("ClassifyThought: %v", err)
	}
	found := false
	for _, tag := range result.Tags {
		if tag.Name == "todo" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'todo' tag, got %v", result.Tags)
	}
}

func TestClassifyThought_ideaText(t *testing.T) {
	p := newTestProvider(t)
	result, err := p.ClassifyThought("what if we replaced the polling loop with a websocket")
	if err != nil {
		t.Fatalf("ClassifyThought: %v", err)
	}
	found := false
	for _, tag := range result.Tags {
		if tag.Name == "idea" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'idea' tag, got %v", result.Tags)
	}
}

func TestClassifyThought_emptyText(t *testing.T) {
	p := newTestProvider(t)
	result, err := p.ClassifyThought("")
	if err != nil {
		t.Fatalf("ClassifyThought: %v", err)
	}
	if len(result.Tags) != 0 {
		t.Errorf("empty text should produce no tags, got %v", result.Tags)
	}
}

// ── AnalyzeSentiment ──────────────────────────────────────────────────────────

func TestAnalyzeSentiment_positive(t *testing.T) {
	p := newTestProvider(t)
	score, err := p.AnalyzeSentiment(context.Background(), "today was absolutely wonderful, I feel so grateful and happy")
	if err != nil {
		t.Fatalf("AnalyzeSentiment: %v", err)
	}
	if score <= 0.1 {
		t.Errorf("positive text: want score > 0.1, got %.4f", score)
	}
}

func TestAnalyzeSentiment_negative(t *testing.T) {
	p := newTestProvider(t)
	score, err := p.AnalyzeSentiment(context.Background(), "everything feels hopeless and I am exhausted and miserable")
	if err != nil {
		t.Fatalf("AnalyzeSentiment: %v", err)
	}
	if score >= -0.1 {
		t.Errorf("negative text: want score < -0.1, got %.4f", score)
	}
}

func TestAnalyzeSentiment_range(t *testing.T) {
	p := newTestProvider(t)
	for _, text := range []string{
		"great day",
		"terrible meeting",
		"the server is down again",
		"shipped the feature",
	} {
		score, err := p.AnalyzeSentiment(context.Background(), text)
		if err != nil {
			t.Fatalf("AnalyzeSentiment(%q): %v", text, err)
		}
		if score < -1 || score > 1 {
			t.Errorf("AnalyzeSentiment(%q) = %.4f outside [-1, 1]", text, score)
		}
	}
}

// ── German language ───────────────────────────────────────────────────────────

func TestClassifyThought_germanTodo(t *testing.T) {
	p := newTestProvider(t)
	result, err := p.ClassifyThought("den Authentifizierungsfehler vor dem Sprint-Ende beheben")
	if err != nil {
		t.Fatalf("ClassifyThought: %v", err)
	}
	found := false
	for _, tag := range result.Tags {
		if tag.Name == "todo" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'todo' tag for German input, got %v", result.Tags)
	}
}

func TestAnalyzeSentiment_germanPositive(t *testing.T) {
	p := newTestProvider(t)
	score, err := p.AnalyzeSentiment(context.Background(), "heute war ein wunderschöner Tag, ich bin so dankbar und glücklich")
	if err != nil {
		t.Fatalf("AnalyzeSentiment: %v", err)
	}
	if score <= 0.1 {
		t.Errorf("German positive text: want score > 0.1, got %.4f", score)
	}
}

func TestAnalyzeSentiment_germanNegative(t *testing.T) {
	p := newTestProvider(t)
	score, err := p.AnalyzeSentiment(context.Background(), "alles fühlt sich hoffnungslos an, ich bin erschöpft und verzweifelt")
	if err != nil {
		t.Fatalf("AnalyzeSentiment: %v", err)
	}
	if score >= -0.1 {
		t.Errorf("German negative text: want score < -0.1, got %.4f", score)
	}
}

// ── Fallback ──────────────────────────────────────────────────────────────────

func TestNewProvider_fallbackWhenLibMissing(t *testing.T) {
	// Simulate missing ONNX Runtime by temporarily breaking the init path.
	// NewProvider() must return a working (stub) provider, never panic.
	_ = os.Setenv("ONNXRUNTIME_LIB_PATH", "/nonexistent/path/libonnxruntime.dylib")
	defer os.Unsetenv("ONNXRUNTIME_LIB_PATH")

	// We cannot easily reset the ortInitOnce in a unit test.
	// Just confirm NewProvider returns non-nil.
	provider := NewProvider()
	if provider == nil {
		t.Fatal("NewProvider returned nil")
	}
}
