# AGIT

AGIT is a small Git wrapper that records AI coding-agent context alongside commits.

The intended flow is:

1. A coding agent records its current context through AGIT's MCP tool.
2. `agit commit` captures the staged Git state.
3. AGIT commits normally, embeds an `Agit-Context-ID` trailer when a commit message is available, and attaches the full context payload as a Git note under `refs/notes/agit`.

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

If the recorded agent context includes a suggested message, `agit commit` uses it automatically. If a message is provided with `-m` or `--message`, AGIT appends an `Agit-Context-ID` trailer so the context ID becomes part of the commit object.

Inspect the context attached to `HEAD`:

```sh
agit debug
```

Push commits and AGIT notes:

```sh
agit push
```

## MCP Setup

Install the AGIT MCP server for Codex:

```sh
agit mcp install codex
```

Install instructions for Claude Code:

```sh
agit mcp install claude
```

The MCP server exposes a `record_context` tool. Coding agents should call that tool before running `agit commit`.

## Notes

Git commit hashes are immutable. AGIT stores the full context as a Git note attached to the commit hash. When a commit message is available, AGIT also embeds the context ID in the commit message so the commit hash commits to that context ID.
