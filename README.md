# ai-git

ai-git is a closed-source CLI that records AI coding-agent context alongside commits.

The intended flow is:

1. A coding agent records its current context through ai-git's MCP tool.
2. `agit commit` captures the staged Git state.
3. ai-git commits normally, embeds an `Agit-Context-ID` trailer when a commit message is available, and attaches the full context payload as a Git note under `refs/notes/agit`.

## Install

ai-git is intended to be installed as a public CLI through npm, Homebrew, or direct binary downloads. The source repository can stay private while public release artifacts are hosted separately.

Install with npm:

```sh
npm install -g agentic-git
```

Install from direct binary download:

```sh
# Linux amd64 example
curl -L https://github.com/Satari4545/ai-git-releases/releases/download/v0.1.0/agit-linux-amd64 -o agit
chmod +x agit
sudo mv agit /usr/local/bin/agit
```

Homebrew support should be added after the first tagged release, once release asset checksums are available. A formula template lives at `packaging/homebrew/agit.rb.template`.

## Usage

Build the CLI:

```sh
go build -o agit.exe ./cmd/agit
```

Record context manually:

```sh
agit mcp record-context --provider codex --intent "Add MCP install command" --message "Add MCP install support" --files "cmd/agit/main.go,internal/agit/mcp_installs.go"
```

Commit with context:

```sh
git add .
agit commit
```

If the recorded agent context includes a suggested message, `agit commit` uses it automatically. If a message is provided with `-m` or `--message`, ai-git appends an `Agit-Context-ID` trailer so the context ID becomes part of the commit object.

Inspect the context attached to `HEAD`:

```sh
agit debug
```

Push commits and ai-git notes:

```sh
agit push
```

## MCP Setup

Install the ai-git MCP server for Codex:

```sh
agit mcp install codex
```

Install instructions for Claude Code:

```sh
agit mcp install claude
```

The MCP server exposes a `record_context` tool. Coding agents should call that tool before running `agit commit`.

## Releasing

Create a tagged release from the private source repository:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Or build release binaries locally:

```powershell
.\scripts\build-release.ps1 -Version v0.1.0
```

The build creates these public binary assets in `dist/`:

- `agit-linux-amd64`
- `agit-linux-arm64`
- `agit-darwin-amd64`
- `agit-darwin-arm64`
- `agit-windows-amd64.exe`
- matching `.sha256` checksum files

Upload those binaries and their `.sha256` files to a matching public release in `Satari4545/ai-git-releases`. For version `0.1.0`, create public release tag `v0.1.0` with these assets:

- `agit-linux-amd64`
- `agit-linux-arm64`
- `agit-darwin-amd64`
- `agit-darwin-arm64`
- `agit-windows-amd64.exe`

Do not push source code to `Satari4545/ai-git-releases`; that repository only needs release assets and optional public-facing release notes.

After public binary assets exist, publish the npm package:

```sh
npm publish
```

The npm package downloads the matching binary during `postinstall`. By default, it expects public binaries in `Satari4545/ai-git-releases` at `https://github.com/Satari4545/ai-git-releases/releases/download/<version>/<asset>`. Set `AIGIT_DOWNLOAD_BASE_URL` when testing a different binary host.

To publish a Homebrew tap, copy `packaging/homebrew/agit.rb.template` into a tap repository as `Formula/agit.rb`, replace `VERSION` and the SHA placeholders with the release values, then install with:

```sh
brew tap Satari4545/ai-git
brew install agit
```

## Notes

Git commit hashes are immutable. ai-git stores the full context as a Git note attached to the commit hash. When a commit message is available, ai-git also embeds the context ID in the commit message so the commit hash commits to that context ID.

## Source Availability

ai-git's source code is private. The npm package installs the CLI binary but does not include the source code.

Copyright (c) 2026 Satari4545. All rights reserved.
