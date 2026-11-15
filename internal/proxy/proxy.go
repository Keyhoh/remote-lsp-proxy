package proxy

import (
	"fmt"

	"github.com/Keyhoh/devcontainer.lsp/internal/backend"
)

// Config contains the local/remote workspace mapping and the backend used to
// start the remote language server process.
type Config struct {
	LocalWorkspace  string
	RemoteWorkspace string
	Backend         backend.Runner
}

// Proxy is the central bridge between the local LSP client and the remote
// language server process.
type Proxy struct {
	cfg Config
}

func New(cfg Config) *Proxy {
	return &Proxy{cfg: cfg}
}

// Run starts the proxy lifecycle. The initial implementation is intentionally
// a minimal scaffold that keeps the architecture aligned with the specification.
func (p *Proxy) Run() error {
	if p.cfg.Backend == nil {
		return fmt.Errorf("remote backend is not configured")
	}

	if p.cfg.LocalWorkspace == "" {
		return fmt.Errorf("local workspace is required")
	}

	if p.cfg.RemoteWorkspace == "" {
		return fmt.Errorf("remote workspace is required")
	}

	return nil
}
