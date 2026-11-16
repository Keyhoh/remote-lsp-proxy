package runtime

import "io"

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
