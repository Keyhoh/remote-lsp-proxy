# Remote LSP Proxy

A protocol-faithful proxy that lets a local editor communicate with a Language Server running in a remote environment while preserving the normal LSP protocol behavior.

This repository is currently a design and implementation project grounded in `SPECIFICATION.md`. The goal is to bridge the gap between a local client such as Neovim and a remote language server process without changing the LSP contract itself.

## Overview

The proxy sits between a local LSP client and a remote Language Server. It relays JSON-RPC messages over stdio, manages the lifecycle of the remote server process, and translates local and remote paths when the client and server refer to the same file with different absolute paths.

The design assumes the following:

- The local editor runs on the developer machine.
- The language server runs in a remote execution environment such as a Dev Container.
- The local and remote environments share the same underlying files, even if their absolute paths differ.
- LSP semantics remain unchanged; only transport, process execution, and path translation are adapted.

## Architecture

```text
┌──────────────────────────────┐
│         Local Host           │
│                              │
│  ┌────────────┐              │
│  │   Neovim   │              │
│  └─────┬──────┘              │
│        │ stdio               │
│  ┌─────▼──────┐              │
│  │   Proxy    │              │
│  └─────┬──────┘              │
│        │                     │
│  ┌─────▼──────┐              │
│  │  Backend   │              │
│  └─────┬──────┘              │
└────────┼─────────────────────┘
         │ Remote Execution
         │
┌────────▼─────────────────────┐
│       Remote Environment     │
│                              │
│  ┌────────────┐              │
│  │ Language   │              │
│  │  Server    │              │
│  └────────────┘              │
│                              │
└──────────────────────────────┘
```

The proxy is started locally and communicates with the remote backend, which launches the language server in the remote environment. The initial backend target is the Dev Container CLI (`devcontainer exec`), but the design is intentionally backend-agnostic.

```text
Neovim
  │
  │ stdio
  ▼
Remote LSP Proxy
  │
  │ devcontainer exec
  ▼
Language Server
```

The proxy and the language server communicate over standard input and output. No dedicated remote TCP listener or long-running sidecar process is required.

## Core Principles

- Do not change the LSP protocol itself.
- Only translate or adapt the transport and environment differences required for remote execution.
- Preserve JSON-RPC 2.0 semantics and message ordering.
- Keep backend details isolated from the core proxy logic.
- Translate only actual `file:` URIs; leave non-file URIs such as `http:` and `https:` untouched.
- Treat file synchronization and remote filesystem proxying as out of scope.
- Assume local and remote files are the same underlying content, but not necessarily at the same absolute path.

## Key Requirements

### 1. Remote process lifecycle management

The proxy must start, monitor, and terminate the remote language server process as needed.

Required responsibilities include:

- launching the remote command
- passing command-line arguments
- setting the remote working directory
- injecting required environment variables
- capturing stdin, stdout, and stderr
- detecting process completion
- handling cleanup after shutdown

The language server process should be managed per client connection, while the remote execution environment itself can be shared across multiple language servers.

### 2. Bidirectional stdio relay

The proxy relays messages between the local client and the remote language server in both directions.

It must:

- support full-duplex communication
- propagate EOF and process exit
- handle buffering and backpressure
- preserve stderr separately from LSP JSON traffic
- clean up resources when communication is interrupted

The client-facing stdout/stderr must not be polluted with non-LSP logs.

### 3. JSON-RPC framing

LSP messages are framed as:

```text
Content-Length: 123\r\n
\r\n
{"jsonrpc":"2.0","method":"..."}
```

The proxy must parse and re-emit `Content-Length` headers correctly and robustly handle:

- split messages across reads
- batched messages in one stream payload
- partial frame boundaries
- serialization of message bodies
- recalculation of `Content-Length` when re-sending data

### 4. File URI translation

When local and remote paths differ, the proxy translates `file:` URIs in LSP messages between the two environments.

Example:

```text
Local:
file:///home/user/project/src/main.ts

Remote:
file:///workspace/project/src/main.ts
```

The translation must be based on workspace mapping, not raw string replacement. The implementation should account for:

- URI decoding/encoding
- platform-specific path representation
- directory boundary correctness
- normalization of paths
- avoiding unintended translation outside the configured workspace roots

Only `file:` URIs are translated. Other URI schemes remain unchanged.

### 5. URI-aware LSP message conversion

The proxy should not hardcode only a few methods. Instead, it should work from the LSP 3.18 data model and translate URIs according to the structure in which they appear.

Examples of URI-bearing objects include:

- `TextDocumentIdentifier`
- `VersionedTextDocumentIdentifier`
- `TextDocumentItem`
- `Location`
- `WorkspaceEdit`
- `WorkspaceFolder`
- `DiagnosticRelatedInformation`
- `DocumentLink`

The proxy also needs to translate URIs in requests, notifications, and responses as they pass through the system, according to the actual communication direction and the URI namespace referenced by the message.

### 6. `processId` handling

The `initialize` request includes a `processId` describing the client process in the local environment. This value is not meaningful in the remote environment and may not even be valid there.

For the initial implementation, the proxy will normalize it to `null` before forwarding the request to the remote language server:

```json
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "params": {
    "processId": null
  }
}
```

The proxy remains responsible for detecting local client shutdown and cleaning up the remote language server process.

### 7. Remote working directory selection

When starting the remote language server, the proxy must set the working directory to the corresponding remote workspace path.

For example:

```text
Local Workspace: /home/user/project
Remote Workspace: /workspace/project
```

The remote language server should be launched with the mapped remote workspace path as its working directory.

## Non-Goals

The initial design intentionally excludes the following:

- file synchronization
- remote filesystem proxying
- transfer of file contents between environments
- maintaining a separate local/remote content mirror
- full remote file system abstraction

The proxy does not provide a new filesystem model. It simply translates the path namespace so the same file can be addressed correctly by both sides.

## Implementation Scope and Constraints

The repository is currently centered on the design of the proxy rather than a full production implementation. The design emphasizes:

- protocol fidelity to LSP and JSON-RPC 2.0
- clean separation of backend concerns from proxy logic
- reliable stdio transport behavior
- correct workspace mapping for file URIs
- minimal impact on the language server and editor behavior

## Dev Container-first Execution Model

The initial runtime target is a Dev Container environment. The remote backend is expected to wrap the language server process by invoking Dev Container tooling such as `devcontainer exec`.

This keeps the design compatible with local development environments while allowing remote execution without introducing a custom remote transport layer.

## Status

This repository is currently a specification-driven foundation for a remote LSP proxy. It does not yet ship a complete implementation; instead, it documents the architecture, protocol assumptions, and the key behavior required for a correct remote LSP bridge.

For the detailed protocol requirements, message mapping rules, and edge cases, refer to `SPECIFICATION.md`.
