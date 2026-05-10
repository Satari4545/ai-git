package main

import (
	"fmt"
	"os"

	"github.com/Satari4545/AGIT/internal/agit"
	gitwrap "github.com/Satari4545/AGIT/internal/git"
)

func main() {
	if len(os.Args) < 2 {
		_ = gitwrap.RunPassthrough(os.Args[1:]...)
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "commit":
		if err := agit.Commit(args); err != nil {
			fmt.Fprintln(os.Stderr, "agit commit error:", err)
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
	case "push":
		if err := agit.Push(args); err != nil {
			fmt.Fprintln(os.Stderr, "agit push error:", err)
			os.Exit(1)
		}	
	default:
		if err := gitwrap.RunPassthrough(os.Args[1:]...); err != nil {
			os.Exit(1)
		}
	}
}