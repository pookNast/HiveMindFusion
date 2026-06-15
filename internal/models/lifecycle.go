package models

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pooknast/HiveMindFusion/internal/config"
	"github.com/pooknast/HiveMindFusion/internal/vram"
)

const defaultContextLen = 4096
const pidDir = "/var/run/hivemind"

// LoadOptions configures a model load operation.
type LoadOptions struct {
	ContextLen int
	SpecNMax   int
}

// Load validates VRAM, then starts llama-server for the named model.
// The model file is resolved as <config.Models.Dir>/<name>[.gguf].
func Load(cfg *config.Config, name string, opts LoadOptions) error {
	if opts.ContextLen == 0 {
		opts.ContextLen = defaultContextLen
	}

	modelPath := resolveModelPath(cfg.Models.Dir, name)
	fi, err := os.Stat(modelPath)
	if err != nil {
		return fmt.Errorf("model %q not found at %s: %w", name, modelPath, err)
	}
	fileSizeMiB := fi.Size() / (1024 * 1024)

	quantType := vram.ParseQuant(filepath.Base(modelPath))
	estimate := vram.Estimate(fileSizeMiB, quantType, opts.ContextLen, vram.KVF16, opts.SpecNMax)
	if !estimate.FitsIn24GB {
		return fmt.Errorf("VRAM check failed for %q: %s", name, estimate.Details)
	}

	// Reject if already running.
	if pid, _ := readPID(name); pid > 0 && isRunning(pid) {
		return fmt.Errorf("model %q already loaded (pid %d)", name, pid)
	}

	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		return fmt.Errorf("failed to create pid dir %s: %w", pidDir, err)
	}

	cmd := BuildCommand(LaunchParams{
		ModelPath:  modelPath,
		ContextLen: opts.ContextLen,
		SpecNMax:   opts.SpecNMax,
	})

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	if err := writePID(name, cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("failed to write pid file: %w", err)
	}

	fmt.Printf("loaded %q (pid %d): %s\n", name, cmd.Process.Pid, estimate.Details)
	return nil
}

// Unload sends SIGTERM to the running llama-server and removes the PID file.
func Unload(name string) error {
	pid, err := readPID(name)
	if err != nil {
		return fmt.Errorf("model %q not loaded (no pid file): %w", name, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = removePID(name)
		return fmt.Errorf("process not found for pid %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("failed to signal pid %d: %w", pid, err)
	}

	_ = removePID(name)
	fmt.Printf("unloaded %q (pid %d)\n", name, pid)
	return nil
}

// Swap atomically replaces the currently-running model with a new one.
// Unloads all running models (best-effort), then loads the requested model.
func Swap(cfg *config.Config, name string, opts LoadOptions) error {
	// Unload all currently running models to free VRAM.
	running, _ := ListRunning()
	for _, r := range running {
		_ = Unload(r)
	}
	return Load(cfg, name, opts)
}

// ListRunning returns the names of all models with active PID files.
func ListRunning() ([]string, error) {
	entries, err := os.ReadDir(pidDir)
	if err != nil {
		return nil, err
	}
	var running []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if len(n) > 4 && n[len(n)-4:] == ".pid" {
			modelName := n[:len(n)-4]
			if pid, _ := readPID(modelName); pid > 0 && isRunning(pid) {
				running = append(running, modelName)
			}
		}
	}
	return running, nil
}

// resolveModelPath returns the full path to a model file.
// Appends ".gguf" if the name has no extension.
func resolveModelPath(dir, name string) string {
	if filepath.Ext(name) == "" {
		name += ".gguf"
	}
	return filepath.Join(dir, name)
}

func readPID(name string) (int, error) {
	data, err := os.ReadFile(pidPath(name))
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid pid in file: %w", err)
	}
	return pid, nil
}

func writePID(name string, pid int) error {
	return os.WriteFile(pidPath(name), []byte(strconv.Itoa(pid)), 0o644)
}

func removePID(name string) error {
	err := os.Remove(pidPath(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func isRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
