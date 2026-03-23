//go:build with_onnx

package onnx

import (
	"fmt"
	"math"
	"sync"

	hftok "github.com/daulet/tokenizers"
	ort "github.com/yalue/onnxruntime_go"
)

const embeddingDim = 384

// tokenizeAndRun tokenises text, runs the ONNX session, and returns a
// normalised 384-dim embedding vector.
func tokenizeAndRun(
	session *ort.DynamicAdvancedSession,
	tok *hftok.Tokenizer,
	mu *sync.Mutex,
	text string,
	outputIsPooled bool,
) ([]float32, error) {
	enc := tok.EncodeWithOptions(text, true,
		hftok.WithReturnAttentionMask(),
		hftok.WithReturnTypeIDs(),
	)
	if len(enc.IDs) == 0 {
		return nil, fmt.Errorf("onnx: tokenizer returned empty encoding for %q", text)
	}

	seqLen := len(enc.IDs)

	// Convert uint32 token IDs → int64 required by the ONNX model.
	inputIDs := make([]int64, seqLen)
	attMask := make([]int64, seqLen)
	typeIDs := make([]int64, seqLen)
	for i, id := range enc.IDs {
		inputIDs[i] = int64(id)
	}
	for i, m := range enc.AttentionMask {
		attMask[i] = int64(m)
	}
	for i, t := range enc.TypeIDs {
		typeIDs[i] = int64(t)
	}

	shape := ort.NewShape(1, int64(seqLen))

	idsTensor, err := ort.NewTensor(shape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("onnx: create ids tensor: %w", err)
	}
	defer idsTensor.Destroy()

	maskTensor, err := ort.NewTensor(shape, attMask)
	if err != nil {
		return nil, fmt.Errorf("onnx: create mask tensor: %w", err)
	}
	defer maskTensor.Destroy()

	typeTensor, err := ort.NewTensor(shape, typeIDs)
	if err != nil {
		return nil, fmt.Errorf("onnx: create type tensor: %w", err)
	}
	defer typeTensor.Destroy()

	mu.Lock()
	defer mu.Unlock()

	if outputIsPooled {
		// Model includes pooling: output is sentence_embedding [1, 384].
		outTensor, err := ort.NewEmptyTensor[float32](ort.NewShape(1, embeddingDim))
		if err != nil {
			return nil, fmt.Errorf("onnx: create output tensor: %w", err)
		}
		defer outTensor.Destroy()

		if err := session.Run(
			[]ort.Value{idsTensor, maskTensor, typeTensor},
			[]ort.Value{outTensor},
		); err != nil {
			return nil, fmt.Errorf("onnx: run session: %w", err)
		}
		out := make([]float32, embeddingDim)
		copy(out, outTensor.GetData())
		return l2Normalize(out), nil
	}

	// Model outputs last_hidden_state [1, seqLen, 384] — apply mean pooling.
	hiddenTensor, err := ort.NewEmptyTensor[float32](ort.NewShape(1, int64(seqLen), embeddingDim))
	if err != nil {
		return nil, fmt.Errorf("onnx: create hidden tensor: %w", err)
	}
	defer hiddenTensor.Destroy()

	if err := session.Run(
		[]ort.Value{idsTensor, maskTensor, typeTensor},
		[]ort.Value{hiddenTensor},
	); err != nil {
		return nil, fmt.Errorf("onnx: run session: %w", err)
	}
	pooled := meanPool(hiddenTensor.GetData(), attMask, seqLen)
	return l2Normalize(pooled), nil
}

// meanPool applies attention-masked mean pooling over last_hidden_state.
// hidden is the flat [1, seqLen, dim] output; mask is 0/1 per token.
func meanPool(hidden []float32, mask []int64, seqLen int) []float32 {
	out := make([]float32, embeddingDim)
	var count float32
	for i := 0; i < seqLen; i++ {
		if mask[i] == 0 {
			continue
		}
		count++
		base := i * embeddingDim
		for d := 0; d < embeddingDim; d++ {
			out[d] += hidden[base+d]
		}
	}
	if count > 0 {
		for d := range out {
			out[d] /= count
		}
	}
	return out
}

// l2Normalize divides vec by its Euclidean norm in-place and returns it.
func l2Normalize(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	norm := float32(math.Sqrt(sum))
	if norm < 1e-9 {
		return vec
	}
	for i := range vec {
		vec[i] /= norm
	}
	return vec
}

// cosineSim returns the dot product of two L2-normalised vectors.
func cosineSim(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
