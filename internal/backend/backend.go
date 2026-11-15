package backend

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Process represents the lifecycle of a remote language server process.
type Process interface {
	Wait() error
	Close() error
	Stdin() io.WriteCloser
	Stdout() io.Reader
	Stderr() io.Reader
}

// Runner starts and manages a process in the remote execution environment.
type Runner interface {
	Start(command string, args []string, dir string, env []string) (Process, error)
}

// DevContainer is the default backend implementation for the initial release.
type DevContainer struct{}

func NewDevContainer() *DevContainer {
	return &DevContainer{}
}

// Start launches a process inside the Dev Container environment using the
// `devcontainer exec` command. This keeps the runtime details isolated from the
// proxy logic while matching the repository's architecture.
func (d *DevContainer) Start(command string, args []string, dir string, env []string) (Process, error) {
	if command == "" {
		return nil, fmt.Errorf("remote command is required")
	}
	if dir == "" {
		return nil, fmt.Errorf("remote workspace is required")
	}

	cmdArgs := []string{"exec", "--workspace-folder", dir, "--", command}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("devcontainer", cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open remote stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open remote stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("open remote stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start remote process: %w", err)
	}

	return &commandProcess{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}, nil
}

type commandProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (p *commandProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *commandProcess) Close() error {
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	return p.Wait()
}

func (p *commandProcess) Stdin() io.WriteCloser {
	return p.stdin
}

func (p *commandProcess) Stdout() io.Reader {
	return p.stdout
}

func (p *commandProcess) Stderr() io.Reader {
	return p.stderr
}
