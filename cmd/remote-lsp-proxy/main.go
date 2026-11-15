package main

import (
	"log"
	"os"

	"github.com/Keyhoh/devcontainer.lsp/internal/backend"
	"github.com/Keyhoh/devcontainer.lsp/internal/proxy"
)

func main() {
	cfg := proxy.Config{
		LocalWorkspace:  ".",
		RemoteWorkspace: ".",
		Backend:         backend.NewDevContainer(),
	}

	p := proxy.New(cfg)
	if err := p.Run(); err != nil {
		log.Printf("remote LSP proxy failed to initialize: %v", err)
		os.Exit(1)
	}

	log.Println("remote LSP proxy initialized")
}
