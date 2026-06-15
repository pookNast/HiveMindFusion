package models

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pook/hivemind/internal/vram"
)

// defaultCtxLen is the context length used for VRAM estimates in list output.
const defaultCtxLen = 8192

// Model holds discovered information about a single GGUF file.
type Model struct {
	Name     string // filename without .gguf
	Filename string
	Path     string
	Quant    string
	SizeMiB  int64
	VRAM     vram.Result
}

// Scan discovers all *.gguf files in dir and returns a Model entry for each.
func Scan(dir string) ([]Model, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("scan %q: %w", dir, err)
	}

	var out []Model
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".gguf") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		sizeMiB := info.Size() / (1024 * 1024)
		quant := vram.ParseQuant(e.Name())
		vramResult := vram.Estimate(sizeMiB, quant, defaultCtxLen, vram.KVQ8, 0)

		out = append(out, Model{
			Name:     strings.TrimSuffix(e.Name(), ".gguf"),
			Filename: e.Name(),
			Path:     filepath.Join(dir, e.Name()),
			Quant:    quant,
			SizeMiB:  sizeMiB,
			VRAM:     vramResult,
		})
	}
	return out, nil
}

// FindByName locates a model in dir matching name (with or without .gguf suffix).
func FindByName(dir, name string) (*Model, error) {
	ms, err := Scan(dir)
	if err != nil {
		return nil, err
	}
	needle := strings.TrimSuffix(strings.ToLower(name), ".gguf")
	for i, m := range ms {
		if strings.ToLower(m.Name) == needle {
			return &ms[i], nil
		}
	}
	return nil, fmt.Errorf("model %q not found in %s", name, dir)
}
