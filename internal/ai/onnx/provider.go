//go:build with_onnx

// Package onnx provides an ai.Provider backed by the
// paraphrase-multilingual-MiniLM-L12-v2 sentence-transformer model running
// locally via ONNX Runtime.  The model supports 50+ languages including
// German and English.
//
// Both the model weights and the ONNX Runtime shared library are embedded in
// the binary; no external installation is required.  The library is extracted
// to the OS cache directory on first run.
//
// Build with: go build -tags with_onnx ./...
// Prerequisites: run scripts/download_ai_deps.sh first.
package onnx

import (
	"context"
	"fmt"
	"log"
	"sync"

	hftok "github.com/daulet/tokenizers"
	ort "github.com/yalue/onnxruntime_go"

	"github.com/thawts/thawts/internal/ai"
)

// inputNames and outputNames for all-MiniLM-L6-v2.
// The Xenova quantized variant includes a pooling layer ("sentence_embedding");
// standard ONNX exports use "last_hidden_state".
var (
	onnxInputNames   = []string{"input_ids", "attention_mask", "token_type_ids"}
	outputPooled     = []string{"sentence_embedding"}
	outputHidden     = []string{"last_hidden_state"}
)

// ONNXProvider implements ai.Provider using on-device ONNX inference.
type ONNXProvider struct {
	mu      sync.Mutex
	session *ort.DynamicAdvancedSession
	tok     *hftok.Tokenizer
	stub    *ai.StubProvider

	outputIsPooled bool // true → "sentence_embedding", false → "last_hidden_state"

	// Anchor embeddings and tag centroids, built lazily on first use.
	anchorOnce   sync.Once
	anchorErr    error
	posAnchor    []float32
	negAnchor    []float32
	tagCentroids map[string][]float32
}

// NewProvider constructs an ONNXProvider, initialising the ONNX Runtime and
// loading the embedded model + tokenizer.  On any error it returns an
// ai.StubProvider so the application always has a working AI backend.
func NewProvider() ai.Provider {
	p, err := newONNXProvider()
	if err != nil {
		log.Printf("ai/onnx: falling back to stub provider: %v", err)
		return ai.NewStubProvider()
	}
	log.Printf("ai/onnx: ONNX provider ready (output=%v)", func() string {
		if p.outputIsPooled {
			return "sentence_embedding"
		}
		return "last_hidden_state+meanpool"
	}())
	return p
}

func newONNXProvider() (*ONNXProvider, error) {
	if err := initORT(); err != nil {
		return nil, err
	}

	tok, err := hftok.FromBytesWithTruncation(tokenizerJSON, 512, hftok.TruncationDirectionRight)
	if err != nil {
		return nil, fmt.Errorf("onnx: load tokenizer: %w", err)
	}

	// Try the pooled output first (Xenova model); fall back to raw hidden states.
	session, pooled, err := openSession(modelONNX)
	if err != nil {
		tok.Close()
		return nil, fmt.Errorf("onnx: open session: %w", err)
	}

	return &ONNXProvider{
		session:        session,
		tok:            tok,
		stub:           ai.NewStubProvider(),
		outputIsPooled: pooled,
	}, nil
}

// openSession opens an ONNX session and probes at runtime to determine
// whether the model exposes a pre-pooled "sentence_embedding" output or the
// raw "last_hidden_state" that requires manual mean-pooling.
//
// The DynamicAdvancedSession does not validate output names at creation time —
// validation only happens during Run() — so we run a one-token probe.
func openSession(data []byte) (*ort.DynamicAdvancedSession, bool, error) {
	newOpts := func() (*ort.SessionOptions, error) {
		o, err := ort.NewSessionOptions()
		if err != nil {
			return nil, err
		}
		// Use CoreML on Apple Silicon for faster inference; ignore errors (optional EP).
		_ = o.AppendExecutionProviderCoreML(0)
		return o, nil
	}

	// Minimal probe inputs: a single [CLS] token (id=101, mask=1, type=0).
	probeShape := ort.NewShape(1, 1)
	probeIDs, _ := ort.NewTensor(probeShape, []int64{101})
	probeMask, _ := ort.NewTensor(probeShape, []int64{1})
	probeType, _ := ort.NewTensor(probeShape, []int64{0})
	defer probeIDs.Destroy()
	defer probeMask.Destroy()
	defer probeType.Destroy()
	probeInputs := []ort.Value{probeIDs, probeMask, probeType}

	// Try sentence_embedding (pooled) first.
	if opts, err := newOpts(); err == nil {
		sess, err := ort.NewDynamicAdvancedSessionWithONNXData(data, onnxInputNames, outputPooled, opts)
		opts.Destroy()
		if err == nil {
			out, _ := ort.NewEmptyTensor[float32](ort.NewShape(1, embeddingDim))
			runErr := sess.Run(probeInputs, []ort.Value{out})
			out.Destroy()
			if runErr == nil {
				return sess, true, nil
			}
			sess.Destroy()
		}
	}

	// Fall back to last_hidden_state with manual mean-pooling.
	opts, err := newOpts()
	if err != nil {
		return nil, false, err
	}
	sess, err := ort.NewDynamicAdvancedSessionWithONNXData(data, onnxInputNames, outputHidden, opts)
	opts.Destroy()
	if err != nil {
		return nil, false, fmt.Errorf("session (pooled and hidden-state both failed): %w", err)
	}
	// Verify with a probe.
	out, _ := ort.NewEmptyTensor[float32](ort.NewShape(1, 1, embeddingDim))
	if runErr := sess.Run(probeInputs, []ort.Value{out}); runErr != nil {
		out.Destroy()
		sess.Destroy()
		return nil, false, fmt.Errorf("session probe failed: %w", runErr)
	}
	out.Destroy()
	return sess, false, nil
}

// embed is the internal helper used by classify.go and sentiment.go.
func (p *ONNXProvider) embed(text string) ([]float32, error) {
	return tokenizeAndRun(p.session, p.tok, &p.mu, text, p.outputIsPooled)
}

// ensureAnchors builds the sentiment anchors and tag centroids on first call.
func (p *ONNXProvider) ensureAnchors() error {
	p.anchorOnce.Do(func() {
		pos, err := meanEmbedding(p, posAnchorPhrases)
		if err != nil {
			p.anchorErr = err
			return
		}
		neg, err := meanEmbedding(p, negAnchorPhrases)
		if err != nil {
			p.anchorErr = err
			return
		}
		centroids, err := buildTagCentroids(p)
		if err != nil {
			p.anchorErr = err
			return
		}
		p.posAnchor = pos
		p.negAnchor = neg
		p.tagCentroids = centroids
	})
	return p.anchorErr
}

// ── ai.Provider interface ─────────────────────────────────────────────────────

// Embed implements ai.Provider.
func (p *ONNXProvider) Embed(_ context.Context, text string) ([]float32, error) {
	return p.embed(text)
}

// ClassifyThought implements ai.Provider.
func (p *ONNXProvider) ClassifyThought(text string) (*ai.Classification, error) {
	if text == "" {
		return &ai.Classification{}, nil
	}
	result, err := classifyWithEmbeddings(p, text)
	if err != nil {
		log.Printf("ai/onnx: classify error, falling back to stub: %v", err)
		return p.stub.ClassifyThought(text)
	}
	// Merge: if embeddings found no tags, fall back to regex to avoid regressions.
	if len(result.Tags) == 0 {
		return p.stub.ClassifyThought(text)
	}
	return result, nil
}

// DetectIntents implements ai.Provider — delegates to stub (regex is reliable here).
func (p *ONNXProvider) DetectIntents(text string) ([]ai.Intent, error) {
	return p.stub.DetectIntents(text)
}

// IsMishap implements ai.Provider — delegates to stub (regex heuristics are effective).
func (p *ONNXProvider) IsMishap(text string, captureMs int64) bool {
	return p.stub.IsMishap(text, captureMs)
}

// AnalyzeSentiment implements ai.Provider.
func (p *ONNXProvider) AnalyzeSentiment(ctx context.Context, text string) (float32, error) {
	score, err := sentimentScore(p, text)
	if err != nil {
		log.Printf("ai/onnx: sentiment error, falling back to stub: %v", err)
		return p.stub.AnalyzeSentiment(ctx, text)
	}
	return score, nil
}

// CleanText implements ai.Provider.
// all-MiniLM-L6-v2 is encoder-only and cannot generate text.
// Text cleaning requires a generative LLM (future work).
func (p *ONNXProvider) CleanText(_ context.Context, text string) (string, error) {
	return text, nil
}
