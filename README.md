# ai-git

ai-git is a closed-source CLI that records AI coding-agent context alongside commits.

The intended flow is:

1. A coding agent records its current context through ai-git's MCP tool.
2. `agit commit` captures the staged Git state.
3. ai-git commits normally, embeds an `Agit-Context-ID` trailer when a commit message is available, and attaches the full context payload as a Git note under `refs/notes/agit`.

New here? Start with [QUICKSTART.md](QUICKSTART.md): 5 minutes to your first agent-to-agent handoff.

## Why This Exists

Commit messages and PR descriptions are useful for humans, but they usually lose the working context an AI coding agent had while making the change. ai-git adds a machine-readable context layer for agents and tools: intent, active files, planned delta, actual delta, decisions, risks, next steps, and branch context.

This direction is supported by recent work on Git-like context management for agents:

- [Git Context Controller: Manage the Context of LLM-based Agents like Git](https://arxiv.org/abs/2508.00031) explores Git-inspired context management for long-running agents.
- [Emergent Mind summary of Git Context Controller](https://www.emergentmind.com/topics/git-context-controller-gcc) summarizes the benchmark gains and agent-memory design.
- [faugustdev/git-context-controller](https://github.com/faugustdev/git-context-controller) is an open-source GCC implementation and useful reference point.

ai-git is not a GCC clone. GCC is agent memory infrastructure; ai-git is a productized Git workflow layer that binds agent context to real commits, checkouts, pushes, and eventually cloud/team timelines.

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

Set up a repo (wires the notes refspec and prints the handoff loop):

```sh
agit init
```

Build the CLI:

```sh
go build -o agit.exe ./cmd/agit
```

Record context manually:

```sh
agit mcp record-context --provider codex --intent "Add MCP install command" --message "Add MCP install support" --files "cmd/agit/main.go,internal/agit/mcp_installs.go" --risks "MCP client config differs by tool" --next-steps "Test install on a clean machine"
```

Check context freshness before committing:

```sh
agit context status
```

Browse context attached to past commits:

```sh
agit context log
agit context log -n 5 -- src/
agit context log --json
```

Commit with context:

```sh
git add .
agit commit
```

When a context provider is configured, `agit commit` asks that provider for fresh agent context before creating the commit. The provider must print JSON to stdout. ai-git saves that context, uses the suggested message when no commit message is provided, appends an `Agit-Context-ID` trailer when possible, and stores the full payload as a Git note.

Configure a context provider:

```sh
agit config context-provider set "your-agent-context-command"
agit config context-timeout 60
```

Agent bridges are pluggable. List them and install one:

```sh
agit bridge list
agit bridge codex install
```

Installing a bridge sets `agit.contextCommand` (used by `agit commit` to ask
the agent for fresh context JSON) and selects it via `git config agit.bridge`.
Checkout handoff (`agit checkout --feed-context`) feeds the restored context
through the active bridge. To switch agents, run that agent's install (setting
`agit.bridge` by hand only changes the checkout feed, not the provider
`agit commit` asks):

```sh
agit bridge <agent> install
```

Configure the Codex bridge:

```sh
agit bridge codex install
agit mcp install codex
```

With that setup, `agit commit` calls `agit bridge codex provider`, which asks Codex to summarize the current session/repo as strict JSON context before the commit is created. The MCP install lets Codex also call `get_git_state`, `get_context_status`, `record_context`, and `get_context` directly.

If multiple Codex sessions exist, pin the bridge to the right one:

```sh
agit bridge codex session set <session-id>
```

### Claude Code

```sh
agit bridge claude-code install
```

This wires `agit commit` to ask Claude Code for context JSON via
`claude -p --output-format json`, selects the bridge, and registers a
native `SessionStart` hook in `~/.claude/settings.json`. After
`agit checkout` restores context, the hook injects it as additional
context at the start of your next Claude Code session in that repo, and
`agit bridge claude-code feed` also writes it to
`.agit/state/claude-code-feed.md` for manual paste.

If no session is configured, the Codex bridge uses `codex exec resume --last`.

Checkout feed prompts by default. To choose a default behavior:

```sh
agit config checkout-feed prompt
agit config checkout-feed enable
agit config checkout-feed disable
```

Then:

```sh
agit checkout feature-branch
```

If the checked-out commit has AGIT context, ai-git restores it and either prompts, feeds automatically, or skips feeding based on config.

Override for one checkout:

```sh
agit checkout feature-branch --feed-context
agit checkout feature-branch --no-feed-context
```

Provider JSON shape:

```json
{
  "provider": "codex",
  "intent": "Add checkout context restore",
  "message": "Restore AGIT context on checkout",
  "files": ["cmd/agit/main.go", "internal/agit/checkout.go"],
  "planned": "Restore saved context after checkout.",
  "actual": "Added checkout restore and MCP get_context.",
  "risks": ["GitHub does not display custom Git notes"],
  "nextSteps": ["Add push delta tracking"]
}
```

If no provider is configured, `agit commit` falls back to context previously recorded through MCP. To commit without context, use `--allow-missing-context`.

During commit, ai-git prints progress so provider calls do not look frozen:

```text
agit: preparing commit context
agit: asking agent provider for context (timeout 60s)
agit: refreshed agent context from provider
agit: capturing staged Git state
agit: creating Git commit
agit: attaching context note
```

Suppress ai-git progress messages with:

```sh
agit commit --quiet
```

Checkout with context:

```sh
agit checkout feature-branch
agit mcp get-context
```

When `agit checkout` lands on a commit with an AGIT note, ai-git restores that note into `.agit/state/checkout.json`. Coding agents can read it through the MCP `get_context` tool.

Inspect the context attached to `HEAD`:

```sh
agit debug
```

Push commits and ai-git notes:

```sh
agit push
```

Fetch commits and ai-git notes (plain `git fetch` also carries notes after `agit init`, but `agit fetch` works regardless):

```sh
agit fetch
```

Pull commits, then fetch ai-git notes:

```sh
agit pull
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

The MCP server exposes:

- `record_context` to save active coding-agent context before `agit commit`.
- `get_context` to read context restored by `agit checkout`.
- `get_git_state` to let agents inspect branch, HEAD, staged files, unstaged files, and untracked files before recording context.
- `get_context_status` to let agents inspect active context freshness before committing.

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
