//go:build with_onnx

package onnx

import "math"

// Anchor phrases used to define the sentiment axis in embedding space.
// Both English and German phrases are used so the axis is well-centred for
// inputs in either language. The multilingual model maps them into the same
// vector space, so averaging EN + DE anchors works correctly.
var posAnchorPhrases = []string{
	"I feel wonderful, happy, and deeply grateful today",
	"Ich fühle mich wunderbar, glücklich und sehr dankbar",
}

var negAnchorPhrases = []string{
	"I feel terrible, anxious, exhausted and completely hopeless",
	"Ich fühle mich schrecklich, ängstlich, erschöpft und völlig hoffnungslos",
}

// sentimentScore computes a polarity score in [-1, +1] for text by projecting
// its embedding onto the axis defined by the pre-computed anchor embeddings.
func sentimentScore(p *ONNXProvider, text string) (float32, error) {
	if err := p.ensureAnchors(); err != nil {
		return 0, err
	}

	emb, err := p.embed(text)
	if err != nil {
		return 0, err
	}

	posS := cosineSim(emb, p.posAnchor)
	negS := cosineSim(emb, p.negAnchor)

	denom := posS + negS
	if denom < 1e-6 {
		return 0, nil
	}
	raw := (posS - negS) / denom
	return float32(math.Max(-1, math.Min(1, float64(raw)))), nil
}

// meanEmbedding embeds a list of phrases and returns the L2-normalised mean.
func meanEmbedding(p *ONNXProvider, phrases []string) ([]float32, error) {
	centroid := make([]float32, embeddingDim)
	for _, phrase := range phrases {
		emb, err := p.embed(phrase)
		if err != nil {
			return nil, err
		}
		for d := range centroid {
			centroid[d] += emb[d]
		}
	}
	n := float32(len(phrases))
	for d := range centroid {
		centroid[d] /= n
	}
	return l2Normalize(centroid), nil
}
