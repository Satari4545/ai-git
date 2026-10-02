# Quickstart: 5 minutes to your first handoff

Record agent context on one machine, pick it up on another. Copy-paste each step.

Prerequisites: `agit` installed (`npm install -g agentic-git`) and a git repo with an `origin` remote.

## 1. Set up the repo

```sh
agit init
```

Expect: agit wires the repo (including a notes refspec on `origin` so plain `git fetch` also carries context notes) and prints this loop.

## 2. Connect your agent

```sh
agit mcp install codex
```

Expect: the agit MCP server is registered for Codex. Restart Codex and run `/mcp` to verify.

Claude Code instead:

```sh
agit mcp install claude
```

Expect: prints the `claude mcp add` command to run.

## 3. Record what the agent is doing

The agent calls the `record_context` MCP tool. Same thing from the CLI:

```sh
agit mcp record-context --provider codex \
  --intent "Fix login redirect loop on expired refresh tokens" \
  --message "Fix login redirect loop" \
  --files "src/auth/session.go,src/auth/middleware.go" \
  --risks "Token refresh race under concurrent requests" \
  --next-steps "Add regression test for expired refresh tokens,Verify against staging"
```

Expect: `Agent context written to .agit/state/current.json`.

## 4. Commit with context

```sh
git add -A
agit commit -m "Fix login redirect loop"
```

Expect: normal git commit, plus an `Agit-Context-ID` trailer in the message and the full context payload stored as a git note under `refs/notes/agit`.

## 5. Share it

```sh
agit push
```

Expect: commits pushed, then `agit: pushed code and logic notes`.

## 6. Pick it up on a second machine or session

```sh
agit fetch
```

Expect: refs fetched, then `agit: fetched logic notes`. (`agit pull` does the same after pulling.)

## 7. Restore the context

```sh
agit checkout <branch>
```

Expect: `agit: restored checkout context from HEAD`, then a feed prompt (configurable via `agit config checkout-feed`).

## 8. Let the agent read it

The agent calls the `get_context` MCP tool and gets back the intent, files, risks, and next steps from step 3. Same thing from the CLI:

```sh
agit mcp get-context
```

That is the handoff: what the first agent knew is now what the second agent knows.

## What just happened

Context travels with the repo as git notes bound to real commits. Nothing leaves your git remote except through the refs you already push and fetch.

Next: `agit context log` to browse past context, `agit bridge <agent> install` so `agit commit` can ask the agent for fresh context automatically. See README.md.
