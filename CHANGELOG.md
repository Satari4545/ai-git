# Changelog

## v0.2.0 (2026-10-02)

First release with the full bridge lineup and handoff UX.

### Agent bridges

- Six bridges: Codex, Claude Code, Pi, Cursor, Gemini CLI, OpenCode.
- `agit bridge <agent> install` wires the agent for context capture
  (`agit commit` asks the agent for fresh context JSON) and checkout feed.
- `agit mcp install <codex|claude|pi|cursor|gemini|opencode>` registers the
  ai-git MCP server with the agent's config.
- Claude Code gets a SessionStart hook that pulls context inside the active
  session; the other bridges feed context through their CLI on checkout.

### Checkout handoff

- `agit checkout` restores the HEAD note into `.agit/state/checkout.json`
  and feeds it to the configured bridge automatically (no prompts, ever).
- Feed behavior is configurable: `agit config checkout-feed
  enable|disable|prompt|status`, with `--feed-context`/`--no-feed-context`
  per-checkout overrides. A failed feed warns and the checkout still exits 0.
- Smart session targeting for Codex and Pi: a pinned session wins, a single
  live session is fed silently, otherwise the most recent session is fed
  with a one-line notice naming it.
- `agit bridge <agent> session list` shows live sessions for the repo with
  recency and a preview; `session set|get|unset` pins one.

### Context capture

- `agit mcp record-context` records structured context (intent, files,
  risks, next steps, decisions, deltas) with secret redaction and size
  limits before it is written.
- MCP tools: `record_context`, `get_context`, `get_git_state`,
  `get_context_status`, `get_history`.
- `agit context log` browses context notes on recent commits, with
  `--json` and path filtering.
- `agit commit` attaches an `Agit-Context-ID` trailer and stores the full
  payload as a git note under `refs/notes/agit`.

### Sync

- `agit push` pushes commits, then pushes ai-git notes.
- `agit fetch` / `agit pull` fetch refs, then fetch ai-git notes.
- `agit init` wires a notes refspec on `origin` so plain `git fetch` also
  carries context notes, and prints the 5-minute handoff loop.

### Operations

- `agit doctor` checks repo setup, notes refspec, bridge, bridge CLI, and
  MCP install, printing PASS/WARN/FAIL per item.
- `agit init` auto-detects installed agent CLIs and suggests the matching
  `agit mcp install` target.
- MCP server hardened: 16 MB message limit, correct JSON-RPC
  notification/error handling; Windows uses `agit.cmd`.
- The npm installer verifies the SHA-256 checksum of every downloaded
  binary and refuses to install on mismatch.

## v0.1.0 (2026-05-10)

Initial release.
