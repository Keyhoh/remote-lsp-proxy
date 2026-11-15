package backend

// Process represents the lifecycle of a remote language server process.
type Process interface {
	Wait() error
	Close() error
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

// Start is intentionally a placeholder for the remote execution implementation.
// The real implementation will launch a process through the Dev Container runtime,
// while keeping backend-specific details isolated from the proxy core.
func (d *DevContainer) Start(command string, args []string, dir string, env []string) (Process, error) {
	_ = command
	_ = args
	_ = dir
	_ = env
	return nil, nil
}
