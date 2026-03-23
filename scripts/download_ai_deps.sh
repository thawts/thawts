#!/usr/bin/env bash
# download_ai_deps.sh — fetches the ONNX model, tokenizer, and native runtime libs
# for the paraphrase-multilingual-MiniLM-L12-v2 sentence-transformer pipeline.
# Supports 50+ languages including German and English.
#
# Usage:
#   bash scripts/download_ai_deps.sh              # detect platform automatically
#   bash scripts/download_ai_deps.sh darwin arm64 # explicit GOOS/GOARCH
#
# Outputs into:
#   internal/ai/onnx/models/   — model.onnx, tokenizer.json
#   internal/ai/onnx/libs/     — libonnxruntime.{dylib,so,dll}, libtokenizers.a
#
# These files are .gitignored (too large for git); run this script before building.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MODELS_DIR="$ROOT/internal/ai/onnx/models"
LIBS_DIR="$ROOT/internal/ai/onnx/libs"

GOOS="${1:-$(go env GOOS)}"
GOARCH="${2:-$(go env GOARCH)}"

ORT_VERSION="1.24.4"
TOKENIZERS_VERSION="1.26.0"
HF_MODEL="sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
HF_BASE="https://huggingface.co/${HF_MODEL}/resolve/main"

echo "==> Platform: ${GOOS}/${GOARCH}"
echo "==> ONNX Runtime: v${ORT_VERSION}"
echo "==> Tokenizers:   v${TOKENIZERS_VERSION}"

# ── helpers ──────────────────────────────────────────────────────────────────

download() {
  local url="$1" dest="$2"
  if [[ -f "$dest" ]]; then
    echo "  already exists: $(basename "$dest")"
    return
  fi
  echo "  downloading: $(basename "$dest")"
  curl -fsSL --progress-bar -o "$dest" "$url"
}

# ── model + tokenizer ─────────────────────────────────────────────────────────

echo ""
echo "==> Downloading model files..."

# Use the ONNX-quantized model from Xenova mirror (reliable, no auth needed)
XENOVA_BASE="https://huggingface.co/Xenova/paraphrase-multilingual-MiniLM-L12-v2/resolve/main"
download "${XENOVA_BASE}/onnx/model_quantized.onnx"  "$MODELS_DIR/model.onnx"
download "${HF_BASE}/tokenizer.json"                  "$MODELS_DIR/tokenizer.json"
download "${HF_BASE}/tokenizer_config.json"           "$MODELS_DIR/tokenizer_config.json"

# ── ONNX Runtime shared library ───────────────────────────────────────────────

echo ""
echo "==> Downloading ONNX Runtime v${ORT_VERSION}..."

ORT_BASE="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}"
TMPDIR_ORT="$(mktemp -d)"

case "${GOOS}/${GOARCH}" in
  darwin/arm64)
    ORT_ARCHIVE="onnxruntime-osx-arm64-${ORT_VERSION}.tgz"
    ORT_LIB_SRC="onnxruntime-osx-arm64-${ORT_VERSION}/lib/libonnxruntime.${ORT_VERSION}.dylib"
    ORT_LIB_DEST="$LIBS_DIR/libonnxruntime_darwin_arm64.dylib"
    ;;
  darwin/amd64)
    ORT_ARCHIVE="onnxruntime-osx-x86_64-${ORT_VERSION}.tgz"
    ORT_LIB_SRC="onnxruntime-osx-x86_64-${ORT_VERSION}/lib/libonnxruntime.${ORT_VERSION}.dylib"
    ORT_LIB_DEST="$LIBS_DIR/libonnxruntime_darwin_amd64.dylib"
    ;;
  linux/amd64)
    ORT_ARCHIVE="onnxruntime-linux-x64-${ORT_VERSION}.tgz"
    ORT_LIB_SRC="onnxruntime-linux-x64-${ORT_VERSION}/lib/libonnxruntime.so.${ORT_VERSION}"
    ORT_LIB_DEST="$LIBS_DIR/libonnxruntime_linux_amd64.so"
    ;;
  linux/arm64)
    ORT_ARCHIVE="onnxruntime-linux-aarch64-${ORT_VERSION}.tgz"
    ORT_LIB_SRC="onnxruntime-linux-aarch64-${ORT_VERSION}/lib/libonnxruntime.so.${ORT_VERSION}"
    ORT_LIB_DEST="$LIBS_DIR/libonnxruntime_linux_arm64.so"
    ;;
  windows/amd64)
    ORT_ARCHIVE="onnxruntime-win-x64-${ORT_VERSION}.zip"
    ORT_LIB_SRC="onnxruntime-win-x64-${ORT_VERSION}/lib/onnxruntime.dll"
    ORT_LIB_DEST="$LIBS_DIR/libonnxruntime_windows_amd64.dll"
    ;;
  *)
    echo "ERROR: unsupported platform ${GOOS}/${GOARCH}" >&2
    exit 1
    ;;
esac

if [[ ! -f "$ORT_LIB_DEST" ]]; then
  echo "  downloading: ${ORT_ARCHIVE}"
  curl -fsSL --progress-bar -o "$TMPDIR_ORT/$ORT_ARCHIVE" "${ORT_BASE}/${ORT_ARCHIVE}"
  echo "  extracting..."
  tar -xzf "$TMPDIR_ORT/$ORT_ARCHIVE" -C "$TMPDIR_ORT"
  cp "$TMPDIR_ORT/$ORT_LIB_SRC" "$ORT_LIB_DEST"
  # Canonical name used by //go:embed (no platform suffix, one per build machine).
  cp "$ORT_LIB_DEST" "$LIBS_DIR/libonnxruntime.dylib" 2>/dev/null || \
    cp "$ORT_LIB_DEST" "$LIBS_DIR/libonnxruntime.so" 2>/dev/null || \
    cp "$ORT_LIB_DEST" "$LIBS_DIR/onnxruntime.dll" 2>/dev/null || true
  echo "  saved: $(basename "$ORT_LIB_DEST")"
else
  echo "  already exists: $(basename "$ORT_LIB_DEST")"
fi
rm -rf "$TMPDIR_ORT"

# ── libtokenizers (HuggingFace Rust tokenizers Go bindings) ───────────────────

echo ""
echo "==> Downloading libtokenizers v${TOKENIZERS_VERSION}..."

TOK_BASE="https://github.com/daulet/tokenizers/releases/download/v${TOKENIZERS_VERSION}"
TMPDIR_TOK="$(mktemp -d)"

case "${GOOS}/${GOARCH}" in
  darwin/arm64)
    TOK_ARCHIVE="libtokenizers.darwin-arm64.tar.gz"
    TOK_LIB_DEST="$LIBS_DIR/libtokenizers_darwin_arm64.a"
    ;;
  darwin/amd64)
    TOK_ARCHIVE="libtokenizers.darwin-amd64.tar.gz"
    TOK_LIB_DEST="$LIBS_DIR/libtokenizers_darwin_amd64.a"
    ;;
  linux/amd64)
    TOK_ARCHIVE="libtokenizers.linux-amd64.tar.gz"
    TOK_LIB_DEST="$LIBS_DIR/libtokenizers_linux_amd64.a"
    ;;
  linux/arm64)
    TOK_ARCHIVE="libtokenizers.linux-arm64.tar.gz"
    TOK_LIB_DEST="$LIBS_DIR/libtokenizers_linux_arm64.a"
    ;;
  windows/amd64)
    TOK_ARCHIVE="libtokenizers.windows-amd64.tar.gz"
    TOK_LIB_DEST="$LIBS_DIR/libtokenizers_windows_amd64.a"
    ;;
  *)
    echo "ERROR: unsupported platform ${GOOS}/${GOARCH}" >&2
    exit 1
    ;;
esac

if [[ ! -f "$TOK_LIB_DEST" ]]; then
  echo "  downloading: ${TOK_ARCHIVE}"
  curl -fsSL --progress-bar -o "$TMPDIR_TOK/$TOK_ARCHIVE" "${TOK_BASE}/${TOK_ARCHIVE}"
  tar -xzf "$TMPDIR_TOK/$TOK_ARCHIVE" -C "$TMPDIR_TOK"
  # The archive contains libtokenizers.a
  # Keep a platform-stamped copy and create the canonical libtokenizers.a the linker expects.
  cp "$TMPDIR_TOK/libtokenizers.a" "$TOK_LIB_DEST"
  cp "$TOK_LIB_DEST" "$LIBS_DIR/libtokenizers.a"
  echo "  saved: $(basename "$TOK_LIB_DEST")"
else
  echo "  already exists: $(basename "$TOK_LIB_DEST")"
fi
rm -rf "$TMPDIR_TOK"

echo ""
echo "==> All AI dependencies ready."
echo "    Models : $MODELS_DIR"
echo "    Libs   : $LIBS_DIR"
echo ""
echo "    Next: go build ./..."
