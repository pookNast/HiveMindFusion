// Package vram estimates GPU VRAM requirements for running LLM models.
package vram

import (
	"fmt"
	"regexp"
	"strings"
)

// KVCacheType represents the quantization of the KV cache.
type KVCacheType int

const (
	KVF16    KVCacheType = iota // fp16 — 1x compression (baseline)
	KVQ8                        // 8-bit — 2x compression
	KVTurbo3                    // turbo3 — 4.6x compression
)

// Result holds the VRAM estimate and fit verdict.
type Result struct {
	EstimateMiB int64
	FitsIn24GB  bool
	Details     string
}

// Architecture defaults for GQA transformer models in the ~27B class.
// These match Qwen2.5/3.x 27B parameter estimates (32 layers, 8 KV heads, 128 head dim).
const (
	defaultLayers  = 32
	defaultKVHeads = 8
	defaultHeadDim = 128
	fp16Bytes      = 2
	limitMiB       = 24 * 1024 // 24 GiB fit threshold
	draftMiBPerN   = 256       // per-speculative-step overhead (activations + misc)
)

// kvCompression is the compression ratio relative to fp16 baseline for each KV cache type.
var kvCompression = map[KVCacheType]float64{
	KVF16:    1.0,
	KVQ8:     2.0,
	KVTurbo3: 4.6,
}

// ParseKVCacheType maps a string identifier to a KVCacheType.
func ParseKVCacheType(s string) (KVCacheType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "f16":
		return KVF16, nil
	case "q8", "q8_0":
		return KVQ8, nil
	case "turbo3":
		return KVTurbo3, nil
	default:
		return KVF16, fmt.Errorf("unknown KV cache type %q (valid: f16, q8, turbo3)", s)
	}
}

var quantRe = regexp.MustCompile(`(?i)(Q\d+_K(?:_[SMLX]+L?)?|Q\d+_\d+|Q\d+|BF16|F16)`)

// ParseQuant extracts the quantization tag from a model filename.
// Example: "Qwen3.6-27B-Q4_K_XL.gguf" → "Q4_K_XL"
func ParseQuant(filename string) string {
	return quantRe.FindString(strings.ToUpper(filename))
}

// Estimate returns the VRAM required to run a model with the given parameters.
//
// Formula:
//
//	total = model_mib + kv_mib + draft_mib
//	kv_mib = 2 * layers * kv_heads * head_dim * fp16_bytes * ctx / kv_compression / 2²⁰
//	draft_mib = specDraftNMax * draftMiBPerN
//
// Parameters:
//   - fileSizeMiB: model file size in MiB (used directly as model VRAM)
//   - quantType:   quantization string parsed from filename (e.g. "Q4_K_XL"); informational
//   - ctxLen:      context window length in tokens
//   - kvType:      KV cache quantization type (KVF16 / KVQ8 / KVTurbo3)
//   - specDraftNMax: maximum speculative draft steps
func Estimate(fileSizeMiB int64, quantType string, ctxLen int, kvType KVCacheType, specDraftNMax int) Result {
	compression, ok := kvCompression[kvType]
	if !ok {
		compression = 1.0
	}

	// KV cache: 2 tensors (K and V) × layers × kv_heads × head_dim × fp16_bytes × ctx_tokens
	kvBytes := float64(2 * defaultLayers * defaultKVHeads * defaultHeadDim * fp16Bytes * ctxLen)
	kvMiB := kvBytes / compression / (1024 * 1024)

	// Per-step overhead for each speculative draft token
	draftMiB := float64(specDraftNMax) * draftMiBPerN

	totalMiB := float64(fileSizeMiB) + kvMiB + draftMiB
	totalMiBInt := int64(totalMiB)
	fits := totalMiB <= float64(limitMiB)

	verdict := "FIT"
	if !fits {
		verdict = "NO-FIT"
	}
	details := fmt.Sprintf(
		"[%s] model=%dMiB kv=%.0fMiB(ctx=%d,%s) draft=%dMiB(n=%d) total=%dMiB limit=%dMiB",
		verdict, fileSizeMiB, kvMiB, ctxLen, kvTypeName(kvType), int64(draftMiB), specDraftNMax, totalMiBInt, limitMiB,
	)

	return Result{
		EstimateMiB: totalMiBInt,
		FitsIn24GB:  fits,
		Details:     details,
	}
}

func kvTypeName(t KVCacheType) string {
	switch t {
	case KVF16:
		return "f16"
	case KVQ8:
		return "q8"
	case KVTurbo3:
		return "turbo3"
	default:
		return "unknown"
	}
}
