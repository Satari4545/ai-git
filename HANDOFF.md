# ai-git Handoff

## Product Direction

ai-git is a private-source CLI product for making AI-assisted code changes auditable.

The public npm package is `agentic-git`, but the installed command is `agit`.

Core wedge:

> Git shows what changed. ai-git preserves why it changed.

The near-term MVP should prove that coding agents can reliably record structured context, bind it to commits, restore it on checkout, and eventually sync it to a cloud UI.

## Current Architecture

- `cmd/agit/main.go` is the CLI entrypoint.
- `internal/agit/commit.go` wraps `git commit`, captures local Git state, embeds `Agit-Context-ID`, and stores full context as a Git note under `refs/notes/agit`.
- `internal/agit/checkout.go` wraps `git checkout`, reads the AGIT note on `HEAD`, and restores it to `.agit/state/checkout.json`.
- `internal/agit/mcp.go` exposes MCP tools:
  - `record_context`
  - `get_context`
  - `get_git_state`
  - `get_context_status`
- `internal/agit/provider.go` runs a configured agent context provider before `agit commit`.
- `internal/agit/bridge_codex.go` provides the first provider bridge: `agit bridge codex install|provider`.
- `internal/agit/config.go` manages repo-local ai-git config.
- `internal/agit/agent.go` writes active agent context to `.agit/state/current.json`.
- `internal/agit/push.go` wraps `git push` and pushes `refs/notes/agit`.
- `internal/state/state.go` captures staged files, status, diff stat, and intent into a logic delta.

## Distribution Model

Source stays private.

Public distribution uses:

- npm package: `agentic-git`
- command: `agit`
- public binary repo: `https://github.com/Satari4545/ai-git-releases`
- proprietary license: `LICENSE`

The npm installer downloads binaries from:

```text
https://github.com/Satari4545/ai-git-releases/releases/download/<version>/<asset>
```

## Release Flow

From the private source repo:

```powershell
go test ./...
.\scripts\build-release.ps1 -Version v0.1.1
```

Upload everything in `dist/` to a matching public release in `Satari4545/ai-git-releases`.

Required assets:

- `agit-linux-amd64`
- `agit-linux-amd64.sha256`
- `agit-linux-arm64`
- `agit-linux-arm64.sha256`
- `agit-darwin-amd64`
- `agit-darwin-amd64.sha256`
- `agit-darwin-arm64`
- `agit-darwin-arm64.sha256`
- `agit-windows-amd64.exe`
- `agit-windows-amd64.exe.sha256`

After assets are public:

```powershell
npm publish
```

If npm asks for 2FA:

```powershell
npm publish --otp <code>
```

## Important Git Notes Behavior

Git notes are not part of the commit object and GitHub does not display custom notes in the normal commit UI.

Current note ref:

```text
refs/notes/agit
```

Manual commands:

```powershell
git notes --ref=agit show HEAD
git push origin refs/notes/agit
git fetch origin refs/notes/agit:refs/notes/agit
```

The visible GitHub-compatible pointer is the commit message trailer:

```text
Agit-Context-ID: ctx_sha256:<hash>
```

Future cloud UI should resolve that ID to a full context page.

## MVP Priorities

1. Improve automatic agent context capture.
2. Harden `agit context status`.
3. Add `agit init` for setup and MCP guidance.
4. Improve `record_context` schema further as real agent integrations mature.
5. Add push delta capture:
   - before SHA
   - after SHA
   - commit list
   - changed files
   - diff stat
   - context IDs in pushed range
6. Add cloud sync later:
   - `agit login`
   - `agit sync`
   - context upload
   - push delta upload

## Agent Context Capture Principle

Do not try to capture private chain-of-thought.

Capture structured, auditable working context:

```json
{
  "provider": "codex",
  "intent": "Restore context on branch checkout",
  "message": "Restore AGIT context on checkout",
  "files": ["cmd/agit/main.go", "internal/agit/checkout.go"],
  "planned": "Restore saved branch context after checkout.",
  "actual": "Added checkout restore and MCP get_context.",
  "risks": ["GitHub does not display Git notes"],
  "nextSteps": ["Add push delta tracking"]
}
```

MCP is the best first integration path. Editor extensions can come later.

## Cloud Direction

Do not start by replacing GitHub.

Start with a context cloud:

- stores context objects by `Context-ID`
- stores push delta events
- links back to GitHub/GitLab/self-hosted commits
- optionally fetches diffs through provider APIs when the user opens the UI

Avoid storing full code diffs by default. That creates security and trust issues. Start with metadata and make full diff sync opt-in.

Long-term positioning:

> AI-native Git context and audit trail for teams.

## Known Risks

- Git notes are not visible on GitHub commit pages.
- Users must push/fetch `refs/notes/agit` for note portability.
- MCP tool calling is agent-dependent; the CLI cannot read hidden agent reasoning.
- Public npm install requires the binary release assets to exist before publish.
- Package name is `agentic-git` because `agit`, `aigit`, and `ai-git` are unavailable or blocked on npm.
- Current project has no meaningful tests yet, only compile/package validation.

## Useful Commands

Record context before commit:

```powershell
go run ./cmd/agit mcp record-context --provider codex --intent "..." --message "..." --files "..."
```

Configure a context provider that `agit commit` calls before committing:

```powershell
go run ./cmd/agit config context-provider set "your-agent-context-command"
```

Pin Codex bridge to a specific session when multiple sessions exist:

```powershell
go run ./cmd/agit bridge codex session set <session-id>
```

Enable opt-in checkout context feed:

```powershell
go run ./cmd/agit config checkout-feed prompt
go run ./cmd/agit config checkout-feed enable
go run ./cmd/agit config checkout-feed disable
```

Commit with context:

```powershell
go run ./cmd/agit commit
```

Checkout and restore context:

```powershell
go run ./cmd/agit checkout main
go run ./cmd/agit mcp get-context
```

Build local release assets:

```powershell
.\scripts\build-release.ps1 -Version v0.1.1
```

Package dry run:

```powershell
npm pack --ignore-scripts --dry-run
```

Install public npm package:

```powershell
npm install -g agentic-git
agit --version
```
