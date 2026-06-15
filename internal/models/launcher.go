package models

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// DefaultPort is the default llama-server HTTP port.
const DefaultPort = 8080

// LaunchParams holds the parameters needed to start llama-server.
type LaunchParams struct {
	ModelPath  string
	ContextLen int
	SpecNMax   int
	Port       int
	Host       string
}

// BuildCommand constructs the llama-server exec.Cmd for the given params.
// Stdout and Stderr are wired to the process's own streams.
func BuildCommand(p LaunchParams) *exec.Cmd {
	if p.Host == "" {
		p.Host = "127.0.0.1"
	}
	if p.Port == 0 {
		p.Port = DefaultPort
	}

	args := []string{
		"--model", p.ModelPath,
		"--host", p.Host,
		"--port", strconv.Itoa(p.Port),
		"--ctx-size", strconv.Itoa(p.ContextLen),
	}
	if p.SpecNMax > 0 {
		args = append(args, "--n-draft", strconv.Itoa(p.SpecNMax))
	}

	cmd := exec.Command("llama-server", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// pidPath returns the PID file path for a named model instance.
func pidPath(name string) string {
	return fmt.Sprintf("/var/run/hivemind/%s.pid", name)
}
