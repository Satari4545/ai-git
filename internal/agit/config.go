package agit

import (
	"fmt"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

func Config(args []string) error {
	if len(args) == 3 && args[0] == "context-provider" && args[1] == "set" {
		if err := gitwrap.Run("config", "agit.contextCommand", args[2]); err != nil {
			return err
		}
		fmt.Println("agit: context provider set")
		return nil
	}

	if len(args) == 2 && args[0] == "context-provider" && args[1] == "get" {
		command, _ := gitwrap.OutputQuiet("config", "--get", "agit.contextCommand")
		if command == "" {
			fmt.Println("No context provider configured.")
			return nil
		}
		fmt.Println(command)
		return nil
	}

	if len(args) == 2 && args[0] == "context-provider" && args[1] == "unset" {
		_ = gitwrap.Run("config", "--unset", "agit.contextCommand")
		fmt.Println("agit: context provider unset")
		return nil
	}

	if len(args) == 2 && args[0] == "checkout-feed" {
		switch args[1] {
		case "enable":
			if err := gitwrap.Run("config", "agit.checkoutFeed", "true"); err != nil {
				return err
			}
			fmt.Println("agit: checkout context feed enabled")
			return nil
		case "disable":
			if err := gitwrap.Run("config", "agit.checkoutFeed", "false"); err != nil {
				return err
			}
			fmt.Println("agit: checkout context feed disabled")
			return nil
		case "prompt":
			if err := gitwrap.Run("config", "agit.checkoutFeed", "prompt"); err != nil {
				return err
			}
			fmt.Println("agit: checkout context feed will prompt")
			return nil
		case "status":
			fmt.Println(CheckoutFeedMode())
			return nil
		}
	}

	if len(args) == 2 && args[0] == "context-timeout" {
		if err := gitwrap.Run("config", "agit.contextTimeoutSec", args[1]); err != nil {
			return err
		}
		fmt.Println("agit: context provider timeout set")
		return nil
	}

	if len(args) == 2 && args[0] == "redact" {
		switch args[1] {
		case "on", "enable", "enabled", "true":
			if err := gitwrap.Run("config", "agit.redact", "true"); err != nil {
				return err
			}
			fmt.Println("agit: secret redaction enabled")
			return nil
		case "off", "disable", "disabled", "false":
			if err := gitwrap.Run("config", "agit.redact", "false"); err != nil {
				return err
			}
			fmt.Println("agit: secret redaction disabled")
			return nil
		case "status":
			fmt.Println(RedactMode())
			return nil
		}
	}

	fmt.Println("usage: agit config context-provider set <command>|get|unset")
	fmt.Println("usage: agit config context-timeout <seconds>")
	fmt.Println("usage: agit config checkout-feed enable|disable|prompt|status")
	fmt.Println("usage: agit config redact on|off|status")
	return nil
}

func CheckoutFeedMode() string {
	value, _ := gitwrap.OutputQuiet("config", "--get", "agit.checkoutFeed")
	switch value {
	case "true", "enable", "enabled":
		return "enabled"
	case "false", "disable", "disabled":
		return "disabled"
	case "prompt":
		return "prompt"
	default:
		return "enabled"
	}
}
