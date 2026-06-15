package models

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/pooknast/HiveMindFusion/internal/config"
	"github.com/pooknast/HiveMindFusion/internal/vram"
)

func TestResolveModelPath(t *testing.T) {
	tests := []struct {
		dir, name, want string
	}{
		{"/models", "qwen3", "/models/qwen3.gguf"},
		{"/models", "qwen3.gguf", "/models/qwen3.gguf"},
		{"/models", "foo.Q4_K_XL.gguf", "/models/foo.Q4_K_XL.gguf"},
	}
	for _, tt := range tests {
		got := resolveModelPath(tt.dir, tt.name)
		if got != tt.want {
			t.Errorf("resolveModelPath(%q, %q) = %q, want %q", tt.dir, tt.name, got, tt.want)
		}
	}
}

func TestLoad_ModelNotFound(t *testing.T) {
	cfg := &config.Config{Models: config.Models{Dir: t.TempDir()}}
	err := Load(cfg, "nonexistent", LoadOptions{})
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestLoad_VRAMReject(t *testing.T) {
	// Create a huge fake model file to trigger VRAM rejection.
	dir := t.TempDir()
	fakeModel := filepath.Join(dir, "huge-model.Q8_0.gguf")
	// Create a sparse 20GiB file (just metadata, no disk space consumed).
	f, err := os.Create(fakeModel)
	if err != nil {
		t.Fatal(err)
	}
	// 20 GiB offset → file reports 20 GiB size.
	if err := f.Truncate(20 * 1024 * 1024 * 1024); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	cfg := &config.Config{Models: config.Models{Dir: dir}}
	// Q8_0 at 20 GiB + large context should not fit in 24 GB.
	err = Load(cfg, "huge-model.Q8_0.gguf", LoadOptions{ContextLen: 131072})
	if err == nil {
		t.Fatal("expected VRAM rejection for oversized model")
	}
	if got := err.Error(); !contains(got, "VRAM check failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_AlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	// Create a small model.
	fakeModel := filepath.Join(dir, "small.Q4_K_M.gguf")
	f, err := os.Create(fakeModel)
	if err != nil {
		t.Fatal(err)
	}
	f.Truncate(5 * 1024 * 1024 * 1024) // 5 GiB
	f.Close()

	// Write a PID file for a process we know is running (our own PID).
	tmpPidDir := t.TempDir()
	// Override pidDir for testing — we test the helper directly.
	pidFile := filepath.Join(tmpPidDir, "small.Q4_K_M.pid")
	os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644)

	// We can't easily test the full Load path without actually spawning,
	// but we verify isRunning reports our own PID as alive.
	if !isRunning(os.Getpid()) {
		t.Fatal("isRunning should return true for our own PID")
	}
}

func TestUnload_NoPidFile(t *testing.T) {
	err := Unload("model-that-does-not-exist-xyz")
	if err == nil {
		t.Fatal("expected error when no PID file")
	}
}

func TestBuildCommand(t *testing.T) {
	cmd := BuildCommand(LaunchParams{
		ModelPath:  "/models/test.gguf",
		ContextLen: 8192,
		SpecNMax:   3,
	})
	args := cmd.Args
	if args[0] != "llama-server" {
		t.Fatalf("expected llama-server, got %s", args[0])
	}
	// Check required flags present.
	wantPairs := map[string]string{
		"--model":    "/models/test.gguf",
		"--ctx-size": "8192",
		"--n-draft":  "3",
		"--host":     "127.0.0.1",
		"--port":     "8080",
	}
	for k, v := range wantPairs {
		found := false
		for i, a := range args {
			if a == k && i+1 < len(args) && args[i+1] == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %s %s in args: %v", k, v, args)
		}
	}
}

func TestBuildCommand_NoDraft(t *testing.T) {
	cmd := BuildCommand(LaunchParams{
		ModelPath:  "/models/test.gguf",
		ContextLen: 4096,
		SpecNMax:   0,
	})
	for _, a := range cmd.Args {
		if a == "--n-draft" {
			t.Fatal("--n-draft should not be present when SpecNMax=0")
		}
	}
}

func TestVRAMEstimate_Integration(t *testing.T) {
	// Verify the VRAM estimator works with lifecycle-relevant params.
	// 5 GiB Q4_K model at 4096 context should fit easily.
	result := vram.Estimate(5120, "Q4_K_M", 4096, vram.KVF16, 0)
	if !result.FitsIn24GB {
		t.Fatalf("5 GiB Q4_K at 4K context should fit: %s", result.Details)
	}
}

func TestListRunning_EmptyDir(t *testing.T) {
	// When pidDir doesn't exist, ListRunning should return empty without error.
	running, _ := ListRunning()
	// With default pidDir (/var/run/hivemind), this may or may not exist.
	// Either way, the function should not panic.
	_ = running
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
