//go:build with_onnx

package onnx

import _ "embed"

// modelONNX is the quantized paraphrase-multilingual-MiniLM-L12-v2 ONNX model (~118 MB).
// Supports 50+ languages including German and English.
// Fetched by scripts/download_ai_deps.sh.
//
//go:embed models/model.onnx
var modelONNX []byte

// tokenizerJSON is the HuggingFace tokenizer spec for all-MiniLM-L6-v2 (~455 KB).
//
//go:embed models/tokenizer.json
var tokenizerJSON []byte
