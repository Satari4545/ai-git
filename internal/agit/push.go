package agit

import (
	"fmt"

	gitwrap "github.com/Satari4545/AGIT/internal/git"
)

func Push(args []string) error {
	if err := gitwrap.Run(append([]string{"push"}, args...)...); err != nil {
		return err
	}

	remote := "origin"
	if len(args) > 0 && args[0] != "" {
		remote = args[0]
	}

	fmt.Println("agit: pushing logic notes")

	if err := gitwrap.Run("push", remote, "refs/notes/agit"); err != nil {
		return err
	}

	fmt.Println("agit: pushed code and logic notes")
	return nil
}