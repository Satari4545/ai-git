package agit

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

func RefreshContextFromProvider(args []string) (bool, error) {
	if hasAgitFlag(args, "--no-agent-context") {
		return false, nil
	}

	command := os.Getenv("AGIT_CONTEXT_COMMAND")
	if command == "" {
		command, _ = gitwrap.OutputQuiet("config", "--get", "agit.contextCommand")
	}

	if command == "" {
		return false, nil
	}

	raw, err := runContextProvider(command)
	if err != nil {
		return false, err
	}

	ctx, err := ParseAgentContextJSON(raw)
	if err != nil {
		return false, fmt.Errorf("invalid agent context provider output: %w", err)
	}

	if err := SaveAgentContext(ctx); err != nil {
		return false, err
	}

	return true, nil
}

func runContextProvider(command string) ([]byte, error) {
	cmd := shellCommand(command)
	cmd.Env = append(os.Environ(), providerEnv()...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			message := strings.TrimSpace(stderr.String())
			if message == "" {
				message = err.Error()
			}
			return nil, fmt.Errorf("agent context provider failed: %s", message)
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("agent context provider timed out after 10s")
	}

	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return nil, fmt.Errorf("agent context provider returned no JSON")
	}

	return raw, nil
}

func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("powershell", "-NoProfile", "-Command", command)
	}

	return exec.Command("sh", "-c", command)
}

func providerEnv() []string {
	gitState := BuildGitState()
	return []string{
		"AGIT_REPO=" + gitState.Repo,
		"AGIT_BRANCH=" + gitState.Branch,
		"AGIT_HEAD=" + gitState.Head,
	}
}
