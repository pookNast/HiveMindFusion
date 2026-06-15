package vram

import (
	"testing"
)

// TestEstimate_Qwen27B_Turbo3_Draft3 verifies the primary acceptance criterion:
// Qwen3.6-27B Q4_K_XL + 160K context + turbo3 KV + 3 draft steps ≈ 20 GB, fits in 24 GB.
//
// Math:
//
//	model  = 15,000 MiB
//	kv     = 2*32*8*128*2*160000 / 4.6 / 1048576 ≈ 4,350 MiB
//	draft  = 3 * 256 = 768 MiB
//	total  ≈ 20,118 MiB ≈ 20 GB
func TestEstimate_Qwen27B_Turbo3_Draft3(t *testing.T) {
	const fileSizeMiB = 15_000 // ~15 GB, typical for 27B at Q4_K_XL
	result := Estimate(fileSizeMiB, "Q4_K_XL", 160_000, KVTurbo3, 3)

	t.Logf("result: %s", result.Details)

	// Must be in the ~20 GB range (±1 GiB tolerance)
	const wantMiB = 20_118
	const tolerance int64 = 1024
	diff := result.EstimateMiB - wantMiB
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		t.Errorf("estimate %d MiB, want ~%d MiB (±%d MiB)", result.EstimateMiB, wantMiB, tolerance)
	}

	// Must fit in 24 GB
	if !result.FitsIn24GB {
		t.Errorf("expected FitsIn24GB=true (got %d MiB, limit %d MiB)", result.EstimateMiB, limitMiB)
	}
}

// TestEstimate_NoFit_Q8_Draft6 verifies the no-fit criterion:
// Same model at 160K context but with q8 KV cache (less compressed) and 6 draft steps
// pushes total VRAM above the 24 GB threshold.
//
// Math:
//
//	model  = 15,000 MiB
//	kv     = 2*32*8*128*2*160000 / 2.0 / 1048576 ≈ 10,000 MiB
//	draft  = 6 * 256 = 1,536 MiB
//	total  ≈ 26,536 MiB > 24,576 MiB → NO-FIT
func TestEstimate_NoFit_Q8_Draft6(t *testing.T) {
	const fileSizeMiB = 15_000
	result := Estimate(fileSizeMiB, "Q4_K_XL", 160_000, KVQ8, 6)

	t.Logf("result: %s", result.Details)

	if result.FitsIn24GB {
		t.Errorf("expected no-fit for n=6 at 160K with q8 KV (got %d MiB, limit %d MiB)",
			result.EstimateMiB, limitMiB)
	}
	if result.EstimateMiB <= limitMiB {
		t.Errorf("estimate %d MiB should exceed limit %d MiB", result.EstimateMiB, limitMiB)
	}
}

// TestEstimate_F16_AlwaysNoFit confirms that full fp16 KV at 160K always exceeds 24 GB
// for a 27B-class model regardless of draft count.
func TestEstimate_F16_AlwaysNoFit(t *testing.T) {
	result := Estimate(15_000, "Q4_K_XL", 160_000, KVF16, 0)
	t.Logf("result: %s", result.Details)
	if result.FitsIn24GB {
		t.Errorf("f16 KV at 160K should not fit (got %d MiB)", result.EstimateMiB)
	}
}

func TestParseQuant(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"Qwen3.6-27B-Q4_K_XL.gguf", "Q4_K_XL"},
		{"llama-3-70b-Q5_K_M.gguf", "Q5_K_M"},
		{"model-Q8_0.gguf", "Q8_0"},
		{"mixtral-8x7b-Q4_K_M.gguf", "Q4_K_M"},
	}
	for _, c := range cases {
		got := ParseQuant(c.filename)
		if got != c.want {
			t.Errorf("ParseQuant(%q) = %q, want %q", c.filename, got, c.want)
		}
	}
}

func TestParseKVCacheType(t *testing.T) {
	cases := []struct {
		input   string
		want    KVCacheType
		wantErr bool
	}{
		{"f16", KVF16, false},
		{"q8", KVQ8, false},
		{"q8_0", KVQ8, false},
		{"turbo3", KVTurbo3, false},
		{"TURBO3", KVTurbo3, false},
		{"unknown", KVF16, true},
	}
	for _, c := range cases {
		got, err := ParseKVCacheType(c.input)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseKVCacheType(%q): err=%v, wantErr=%v", c.input, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseKVCacheType(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

// TestEstimate_KVCompressionRatios validates the relative scaling of KV types.
func TestEstimate_KVCompressionRatios(t *testing.T) {
	const fileSizeMiB = 0 // isolate KV contribution
	r16 := Estimate(fileSizeMiB, "", 160_000, KVF16, 0)
	r8 := Estimate(fileSizeMiB, "", 160_000, KVQ8, 0)
	r3 := Estimate(fileSizeMiB, "", 160_000, KVTurbo3, 0)

	// q8 should be ~2x smaller than f16
	ratio8 := float64(r16.EstimateMiB) / float64(r8.EstimateMiB)
	if ratio8 < 1.9 || ratio8 > 2.1 {
		t.Errorf("f16/q8 ratio = %.2f, want ~2.0", ratio8)
	}

	// turbo3 should be ~4.6x smaller than f16
	ratio3 := float64(r16.EstimateMiB) / float64(r3.EstimateMiB)
	if ratio3 < 4.5 || ratio3 > 4.7 {
		t.Errorf("f16/turbo3 ratio = %.2f, want ~4.6", ratio3)
	}
}
