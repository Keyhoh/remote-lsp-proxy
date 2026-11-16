# Remote LSP Proxy

A protocol-faithful LSP proxy that lets a local editor communicate with a Language Server running in a remote environment while preserving the normal LSP/JSON-RPC behavior.

This project exposes a local stdio endpoint, translates `file:` URIs between local and remote workspaces, and forwards traffic to a language server launched in a remote backend such as a Dev Container.

## Overview

The proxy sits between a local LSP client and a remote language server. It relays JSON-RPC traffic over stdio, manages the remote process lifecycle, and maps workspace paths so a local file URI and a remote file URI can point to the same underlying file.

Typical flow:

```text
Neovim / editor
      │
      │ stdio
      ▼
Remote LSP Proxy
      │
      │ devcontainer exec
      ▼
Remote language server
```

The default backend uses the Dev Container CLI (`devcontainer exec`), but the proxy is designed around a backend interface so a different remote execution strategy can be plugged in later.

## Features

- Bi-directional stdio relay between local client and remote LSP server
- JSON-RPC framing handling for split and batched messages
- `file:` URI translation based on local/remote workspace mappings
- Remote process lifecycle management for language servers
- Clean separation between proxy logic and runtime/backend details

## Prerequisites

- Go toolchain
- `devcontainer` CLI installed and available in PATH
- Optional but recommended: `task` for easier local commands

## Build

Build the proxy binary:

```bash
# with Task
task build

# or directly with Go
go build -o remote-lsp-proxy ./cmd/remote-lsp-proxy
```

This produces a binary named `remote-lsp-proxy`.

## Usage

Start the proxy with the local workspace root, the corresponding remote workspace root, and the remote command you want to execute.

```bash
./remote-lsp-proxy \
  --local-workspace /home/user/project \
  --remote-workspace /workspace/project \
  --remote-command gopls \
  --remote-args "serve"
```

Or using Task:

```bash
task run -- \
  --local-workspace /home/user/project \
  --remote-workspace /workspace/project \
  --remote-command gopls \
  --remote-args "serve"
```

### Required arguments

- `--local-workspace`: required; host filesystem root for the project; this is the path the local editor and the host machine use
- `--remote-workspace`: required; remote filesystem root where the language server should run inside the execution environment
- `--remote-command`: required; executable to launch remotely (for example `gopls`, `clangd`, `rust-analyzer`)
- `--remote-args`: optional quoted arguments passed to the remote command

Important: for the default Dev Container backend, `devcontainer exec --workspace-folder` must receive the host-side workspace path, not the container-side path. The container-side path is still used for URI mapping and remote execution, but it should not be used as the local process working directory.

Example with a local repo mounted in a Dev Container:

```bash
./remote-lsp-proxy \
  --local-workspace "$PWD" \
  --remote-workspace /workspaces/devcontainer.lsp \
  --remote-command gopls \
  --remote-args "serve"
```

In a Dev Container, `devcontainer exec --workspace-folder "$PWD"` resolves the correct container workspace before invoking the language server. The value used for `--local-workspace` must be valid on the host machine.

## How it works

1. The local editor connects to the proxy over stdio.
2. The proxy normalizes and forwards messages according to JSON-RPC framing rules.
3. The proxy translates `file:` URIs from the local workspace namespace to the remote workspace namespace and vice versa.
4. The remote language server is launched through the configured backend (`devcontainer exec` by default).
5. Responses are relayed back to the editor without changing the LSP protocol contract.

## Notes

- This project is intended for editors or tools that launch LSP servers through stdio.
- The proxy does not synchronize files or emulate a remote filesystem; it only translates path namespaces.
- The current implementation is a practical foundation for remote LSP execution and is still designed to remain backend-agnostic.

For deeper protocol and design details, see `SPECIFICATION.md`.

## Status

The repository includes the proxy implementation and the default Dev Container-backed runtime integration, while still keeping the design grounded in the specification and backend abstraction described in `SPECIFICATION.md`.

The initial runtime target is a Dev Container environment. The remote backend is expected to wrap the language server process by invoking Dev Container tooling such as `devcontainer exec`.

This keeps the design compatible with local development environments while allowing remote execution without introducing a custom remote transport layer.

This repository is currently a specification-driven foundation for a remote LSP proxy. It does not yet ship a complete production implementation; instead, it documents the architecture, protocol assumptions, and the key behavior required for a correct remote LSP bridge.

For the detailed protocol requirements, message mapping rules, and edge cases, refer to `SPECIFICATION.md`.
