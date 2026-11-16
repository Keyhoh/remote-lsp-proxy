package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Keyhoh/devcontainer.lsp/internal/backend"
	"github.com/Keyhoh/devcontainer.lsp/internal/proxy"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("remote-lsp-proxy", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var (
		localWorkspace  = flags.String("local-workspace", "", "local workspace root")
		remoteWorkspace = flags.String("remote-workspace", "", "remote workspace root")
		command         = flags.String("remote-command", "", "language server command to run remotely")
		argsString      = flags.String("remote-args", "", "space-delimited arguments for the remote command")
	)
	flags.Usage = func() {
		fmt.Fprintln(stdout, "Usage: remote-lsp-proxy [options]")
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Options:")
		flags.SetOutput(stdout)
		flags.PrintDefaults()
		flags.SetOutput(io.Discard)
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "failed to parse arguments: %v\n", err)
		return 2
	}

	if *localWorkspace == "" {
		fmt.Fprintln(stderr, "local-workspace is required; pass the host workspace path")
		return 1
	}
	if *remoteWorkspace == "" {
		fmt.Fprintln(stderr, "remote-workspace is required; pass the workspace path inside the remote environment")
		return 1
	}
	if *command == "" {
		fmt.Fprintln(stderr, "remote-command is required; pass the language server command to run remotely")
		return 1
	}

	cfg := proxy.Config{
		LocalWorkspace:  *localWorkspace,
		RemoteWorkspace: *remoteWorkspace,
		Backend:         backend.NewDevContainer(),
		Command:         *command,
		Args:            splitArgs(*argsString),
	}

	p := proxy.New(cfg)
	if err := p.Run(); err != nil {
		fmt.Fprintf(stderr, "remote LSP proxy failed to initialize: %v\n", err)
		return 1
	}

	fmt.Fprintln(stderr, "remote LSP proxy initialized")
	return 0
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
