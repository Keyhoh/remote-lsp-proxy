# Copilot instructions for devcontainer.lsp

## Repository context

This repository is a design-and-implementation project for a Remote LSP Proxy. The authoritative project description is `SPECIFICATION.md`, and the current repository is still primarily a spec and Dev Container setup rather than a large application codebase.

The container environment is defined in `.devcontainer/devcontainer.json` and uses the Go devcontainer image with GitHub CLI, Git, and Copilot CLI features preinstalled.

## Build, test, and lint commands

There are currently no checked-in build, test, lint, or package-manager scripts in this repository.

Before adding or changing project commands, check the repo root and `.devcontainer` configuration for the actual automation that exists. Do not invent or assume a build/test pipeline that is not present in the project files.

## High-level architecture

The overall architecture is a local editor connected to a remote language server through a proxy:

- Local editor/Neovim client
- Remote LSP Proxy (this project)
- Dev Container / remote execution environment
- Remote language server process

The architecture described in `SPECIFICATION.md` is intentionally minimal and protocol-focused:

- The LSP protocol itself remains unchanged; the proxy only mediates the execution environment and path differences.
- The proxy relays bidirectional stdio between the local client and the remote language server.
- Remote execution is abstracted behind a backend boundary; the Dev Container CLI (`devcontainer exec`) is the expected initial backend.
- Local and remote files may point to the same underlying file with different absolute paths; the proxy must translate between them.
- No file synchronization or remote filesystem proxying is part of the design.

The design also treats the following as core responsibilities:

- JSON-RPC framing using `Content-Length` headers
- message boundary handling for split or batched LSP messages
- bidirectional `file:` URI translation based on workspace mapping rather than raw string replacement
- preserving stderr/logging separation from LSP JSON traffic

## Key conventions and repo-specific patterns

- Treat `SPECIFICATION.md` as the source of truth for architecture and constraints. If a change does not align with the spec, validate the reason before proceeding.
- Keep the proxy protocol-agnostic: do not change LSP semantics, only adapt transport and path differences where needed.
- Separate protocol handling from execution/backend concerns. Remote environment details should be isolated behind backend logic, not mixed into the core proxy logic.
- Use workspace mapping for `file:` URIs; only translate actual file URIs and leave non-file URIs such as `http:` and `https:` untouched.
- Design around stdio streams and JSON-RPC framing, not around custom transport wrappers or ad hoc message formats.
- Keep LSP traffic on stdout/stdin and avoid writing non-LSP logs to the client-facing stdio stream.
- Assume the local and remote filesystem are shared at the file level, but not path-identical; convert paths rather than synchronizing content.
- The Dev Container setup is a supporting environment, not the product itself. Keep changes compatible with the devcontainer-first workflow used here.

## Working assumptions for future implementation

This repository is currently centered on the backend/proxy design, so future code should preserve the following expectations:

- A minimal, protocol-faithful LSP proxy rather than a custom protocol implementation
- Clear handling of remote/local path transformations
- Stable JSON-RPC framing behavior for split, buffered, and batched messages
- A clean separation between language-server lifecycle management and the Dev Container runtime details

