package vram

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GPUInfo holds real-time VRAM metrics for a single GPU.
type GPUInfo struct {
	Index    int
	UsedMiB  int64
	FreeMiB  int64
	TotalMiB int64
}

// QueryGPUs returns VRAM stats for all available GPUs via nvidia-smi.
func QueryGPUs() ([]GPUInfo, error) {
	out, err := exec.Command(
		"nvidia-smi",
		"--query-gpu=memory.used,memory.free,memory.total",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi failed: %w", err)
	}
	return parseNvidiaSMIOutput(string(out))
}

func parseNvidiaSMIOutput(s string) ([]GPUInfo, error) {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	gpus := make([]GPUInfo, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return nil, fmt.Errorf("unexpected nvidia-smi output line %d: %q", i, line)
		}
		used, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse used MiB: %w", err)
		}
		free, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse free MiB: %w", err)
		}
		total, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse total MiB: %w", err)
		}
		gpus = append(gpus, GPUInfo{
			Index:    i,
			UsedMiB:  used,
			FreeMiB:  free,
			TotalMiB: total,
		})
	}
	if len(gpus) == 0 {
		return nil, fmt.Errorf("no GPUs reported by nvidia-smi")
	}
	return gpus, nil
}
