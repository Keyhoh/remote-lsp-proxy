package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/Keyhoh/devcontainer.lsp/internal/backend"
	"github.com/Keyhoh/devcontainer.lsp/internal/proxy"
)

func main() {
	var (
		localWorkspace  = flag.String("local-workspace", ".", "local workspace root")
		remoteWorkspace = flag.String("remote-workspace", ".", "remote workspace root")
		command         = flag.String("remote-command", "", "language server command to run remotely")
		argsString      = flag.String("remote-args", "", "space-delimited arguments for the remote command")
	)
	flag.Parse()

	cfg := proxy.Config{
		LocalWorkspace:  *localWorkspace,
		RemoteWorkspace: *remoteWorkspace,
		Backend:         backend.NewDevContainer(),
		Command:         *command,
		Args:            splitArgs(*argsString),
	}
	if cfg.Command == "" {
		log.Println("remote command is required; pass --remote-command and optional --remote-args")
		os.Exit(1)
	}

	p := proxy.New(cfg)
	if err := p.Run(); err != nil {
		log.Printf("remote LSP proxy failed to initialize: %v", err)
		os.Exit(1)
	}

	log.Println("remote LSP proxy initialized")
}

func splitArgs(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return nil
	}
	return parts
}
