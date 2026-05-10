# ai-git

ai-git is a small Git wrapper that records AI coding-agent context alongside commits.

The intended flow is:

1. A coding agent records its current context through ai-git's MCP tool.
2. `agit commit` captures the staged Git state.
3. ai-git commits normally, embeds an `Agit-Context-ID` trailer when a commit message is available, and attaches the full context payload as a Git note under `refs/notes/agit`.

## Install

ai-git is intended to be installed as a public CLI through npm, Homebrew, or direct GitHub Release binaries.

Install with npm:

```sh
npm install -g ai-git
```

Install from source:

```sh
go install github.com/Satari4545/ai-git/cmd/agit@latest
```

Install from a GitHub Release:

```sh
# Linux amd64 example
curl -L https://github.com/Satari4545/ai-git/releases/latest/download/agit-linux-amd64 -o agit
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

Create a tagged release:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The GitHub Actions release workflow builds binaries for:

- Linux amd64
- Linux arm64
- macOS Intel
- macOS Apple Silicon
- Windows amd64

After the GitHub Release exists, publish the npm package:

```sh
npm publish
```

The npm package downloads the matching binary from the GitHub Release during `postinstall`.

To publish a Homebrew tap, copy `packaging/homebrew/agit.rb.template` into a tap repository as `Formula/agit.rb`, replace `VERSION` and the SHA placeholders with the release values, then install with:

```sh
brew tap Satari4545/agit
brew install agit
```

## Notes

Git commit hashes are immutable. ai-git stores the full context as a Git note attached to the commit hash. When a commit message is available, ai-git also embeds the context ID in the commit message so the commit hash commits to that context ID.

## License

ai-git is licensed under the MIT License.

Copyright (c) 2026 Satari4545.
