package main

import (
	"fmt"
	"os"

	"github.com/Satari4545/ai-git/internal/agit"
	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "-h", "--help", "help":
		printHelp()
	case "-v", "--version", "version":
		fmt.Println("agit", version)
	case "commit":
		if err := agit.Commit(args); err != nil {
			fmt.Fprintln(os.Stderr, "agit commit error:", err)
			os.Exit(1)
		}
	case "checkout":
		if err := agit.Checkout(args); err != nil {
			fmt.Fprintln(os.Stderr, "agit checkout error:", err)
			os.Exit(1)
		}
	case "debug":
		if err := agit.DebugHEAD(); err != nil {
			fmt.Fprintln(os.Stderr, "agit debug error:", err)
			os.Exit(1)
		}
	case "agent":
		if len(args) > 0 && args[0] == "write" {
			if err := agit.AgentWrite(args[1:]); err != nil {
				fmt.Fprintln(os.Stderr, "agit agent write error:", err)
				os.Exit(1)
			}
		} else {
			fmt.Println("usage: agit agent write --intent ... --message ...")
		}
	case "context":
		if len(args) > 0 && args[0] == "status" {
			if err := agit.PrintContextStatus(); err != nil {
				fmt.Fprintln(os.Stderr, "agit context status error:", err)
				os.Exit(1)
			}
		} else if len(args) > 0 && args[0] == "log" {
			if err := agit.ContextLog(args[1:]); err != nil {
				fmt.Fprintln(os.Stderr, "agit context log error:", err)
				os.Exit(1)
			}
		} else {
			fmt.Println("usage: agit context status|log [-n N] [--json] [-- <path>...]")
		}
	case "bridge":
		if err := agit.Bridge(args); err != nil {
			fmt.Fprintln(os.Stderr, "agit bridge error:", err)
			os.Exit(1)
		}
	case "config":
		if err := agit.Config(args); err != nil {
			fmt.Fprintln(os.Stderr, "agit config error:", err)
			os.Exit(1)
		}
	case "push":
		if err := agit.Push(args); err != nil {
			fmt.Fprintln(os.Stderr, "agit push error:", err)
			os.Exit(1)
		}
	case "fetch":
		if err := agit.Fetch(args); err != nil {
			fmt.Fprintln(os.Stderr, "agit fetch error:", err)
			os.Exit(1)
		}
	case "pull":
		if err := agit.Pull(args); err != nil {
			fmt.Fprintln(os.Stderr, "agit pull error:", err)
			os.Exit(1)
		}
	case "mcp":
		if len(args) > 0 && args[0] == "record-context" {
			if err := agit.AgentWrite(args[1:]); err != nil {
				fmt.Fprintln(os.Stderr, "agit mcp record-context error:", err)
				os.Exit(1)
			}
		} else if len(args) > 0 && args[0] == "get-context" {
			if err := agit.PrintCheckoutContext(); err != nil {
				fmt.Fprintln(os.Stderr, "agit mcp get-context error:", err)
				os.Exit(1)
			}
		} else if len(args) > 0 && args[0] == "serve" {
			if err := agit.MCPServe(); err != nil {
				fmt.Fprintln(os.Stderr, "agit mcp serve error:", err)
				os.Exit(1)
			}
		} else if len(args) > 1 && args[0] == "install" {
			if err := agit.MCPInstall(args[1]); err != nil {
				fmt.Fprintln(os.Stderr, "agit mcp install error:", err)
				os.Exit(1)
			}
		} else {
			fmt.Println("usage: agit mcp record-context|get-context|serve|install <codex|claude>")
		}
	default:
		if err := gitwrap.RunPassthrough(os.Args[1:]...); err != nil {
			os.Exit(1)
		}
	}
}

func printHelp() {
	fmt.Println(`agit - Git with AI coding-agent context

Usage:
  agit <git-command> [args...]
  agit commit [git commit args...]
  agit checkout [--feed-context|--no-feed-context] [git checkout args...]
  agit context status
  agit context log [-n N] [--json] [-- <path>...]
  agit bridge list
  agit bridge <agent> install
  agit bridge codex session set <id>
  agit config checkout-feed prompt|enable|disable
  agit config context-timeout <seconds>
  agit config context-provider set <command>
  agit push [git push args...]
  agit fetch [git fetch args...]
  agit pull [git pull args...]
  agit debug
  agit mcp record-context --intent <summary> [--message <message>]
  agit mcp get-context
  agit mcp serve
  agit mcp install <codex|claude>

Commands:
  commit              Commit staged changes and attach AGIT context metadata.
  checkout            Checkout a branch and restore AGIT context from HEAD.
  context status      Show active agent context and Git state.
  context log         Show AGIT context notes attached to recent commits.
  bridge              Configure provider bridges for coding agents.
  config              Configure AGIT behavior for this repo.
  push                Push Git commits, then push AGIT Git notes.
  fetch               Fetch Git refs, then fetch AGIT Git notes.
  pull                Pull Git refs, then fetch AGIT Git notes.
  debug               Show the AGIT note attached to HEAD.
  mcp record-context  Record coding-agent context for the next commit.
  mcp get-context     Print context restored by the last AGIT checkout.
  mcp serve           Run AGIT's MCP stdio server.
  mcp install         Configure AGIT MCP integration for a supported client.
  version             Print the AGIT version.

Any unknown command is passed through to git.`)
}
