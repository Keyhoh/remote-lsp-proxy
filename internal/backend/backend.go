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
// `devcontainer exec` command. `dir` is the host workspace path that identifies
// the correct devcontainer; the command itself should run inside that container
// and must not set the local process working directory to a remote path.
func (d *DevContainer) Start(command string, args []string, dir string, env []string) (Process, error) {
	if command == "" {
		return nil, fmt.Errorf("remote command is required")
	}
	if dir == "" {
		return nil, fmt.Errorf("workspace root is required")
	}

	cmdArgs := []string{"exec", "--workspace-folder", dir, "--", command}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("devcontainer", cmdArgs...)
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
	var errs []error
	
	// Close stdin to signal EOF to the process
	if p.stdin != nil {
		if err := p.stdin.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	// Give the process time to exit gracefully
	if p.cmd.ProcessState == nil {
		// Process is still running, wait for it to exit on its own
		if err := p.cmd.Wait(); err != nil {
			errs = append(errs, err)
		}
	}
	
	// Only kill if still running after Wait()
	if p.cmd.Process != nil && p.cmd.ProcessState == nil {
		if err := p.cmd.Process.Kill(); err != nil {
			errs = append(errs, err)
		}
		if err := p.cmd.Wait(); err != nil {
			errs = append(errs, err)
		}
	}
	
	// Return first error if any
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
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
