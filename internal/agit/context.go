package agit

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

type ContextStatus struct {
	HasActiveContext    bool          `json:"hasActiveContext"`
	ActiveContext       *AgentContext `json:"activeContext,omitempty"`
	ActiveContextAge    string        `json:"activeContextAge,omitempty"`
	ActiveContextFresh  bool          `json:"activeContextFresh"`
	HasCheckoutContext  bool          `json:"hasCheckoutContext"`
	CheckoutContextPath string        `json:"checkoutContextPath,omitempty"`
	GitState            GitState      `json:"gitState"`
}

type GitState struct {
	Repo       string   `json:"repo,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	Head       string   `json:"head,omitempty"`
	Status     string   `json:"status,omitempty"`
	Staged     []string `json:"staged,omitempty"`
	Unstaged   []string `json:"unstaged,omitempty"`
	Untracked  []string `json:"untracked,omitempty"`
	RemoteHead string   `json:"remoteHead,omitempty"`
}

func PrintContextStatus() error {
	status, err := BuildContextStatus()
	if err != nil {
		return err
	}

	fmt.Println("AGIT CONTEXT STATUS")
	fmt.Println("Repo:", fallback(status.GitState.Repo, "unknown"))
	fmt.Println("Branch:", fallback(status.GitState.Branch, "detached-or-unknown"))
	fmt.Println("HEAD:", fallback(status.GitState.Head, "unknown"))

	if !status.HasActiveContext {
		fmt.Println("Active context: missing")
	} else {
		fmt.Println("Active context: present")
		fmt.Println("Provider:", status.ActiveContext.Provider)
		fmt.Println("Intent:", status.ActiveContext.IntentSummary)
		fmt.Println("Age:", status.ActiveContextAge)
		fmt.Println("Fresh:", status.ActiveContextFresh)
		if len(status.ActiveContext.ActiveFiles) > 0 {
			fmt.Println("Files:", strings.Join(status.ActiveContext.ActiveFiles, ", "))
		}
	}

	if status.HasCheckoutContext {
		fmt.Println("Checkout context:", status.CheckoutContextPath)
	} else {
		fmt.Println("Checkout context: missing")
	}

	if status.GitState.Status != "" {
		fmt.Println()
		fmt.Println("Git status:")
		fmt.Println(status.GitState.Status)
	}

	return nil
}

func BuildContextStatus() (*ContextStatus, error) {
	gitState := BuildGitState()
	activeCtx, hasActiveCtx, err := LoadAgentContext()
	if err != nil {
		return nil, err
	}

	_, hasCheckoutCtx, err := LoadCheckoutContextRaw()
	if err != nil {
		return nil, err
	}

	status := &ContextStatus{
		HasActiveContext:   hasActiveCtx,
		ActiveContext:      activeCtx,
		ActiveContextFresh: false,
		HasCheckoutContext: hasCheckoutCtx,
		GitState:           gitState,
	}

	if hasCheckoutCtx {
		status.CheckoutContextPath = ".agit/state/checkout.json"
	}

	if hasActiveCtx && activeCtx.CreatedAt != "" {
		createdAt, err := time.Parse(time.RFC3339, activeCtx.CreatedAt)
		if err == nil {
			age := time.Since(createdAt)
			status.ActiveContextAge = age.Round(time.Second).String()
			status.ActiveContextFresh = age <= 10*time.Minute
		}
	}

	return status, nil
}

func BuildGitState() GitState {
	repo, _ := gitwrap.OutputQuiet("config", "--get", "remote.origin.url")
	branch, _ := gitwrap.OutputQuiet("branch", "--show-current")
	head, _ := gitwrap.OutputQuiet("rev-parse", "HEAD")
	status, _ := gitwrap.OutputQuiet("status", "--short")
	remoteHead := ""
	if branch != "" {
		remoteHead, _ = gitwrap.OutputQuiet("rev-parse", "origin/"+branch)
	}

	return GitState{
		Repo:       repo,
		Branch:     branch,
		Head:       head,
		Status:     status,
		Staged:     gitLinesQuiet("diff", "--name-only", "--cached"),
		Unstaged:   gitLinesQuiet("diff", "--name-only"),
		Untracked:  gitLinesQuiet("ls-files", "--others", "--exclude-standard"),
		RemoteHead: remoteHead,
	}
}

func PrintGitStateJSON() error {
	raw, err := json.MarshalIndent(BuildGitState(), "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(raw))
	return nil
}

func gitLinesQuiet(args ...string) []string {
	out, err := gitwrap.OutputQuiet(args...)
	if err != nil || out == "" {
		return []string{}
	}

	return strings.Split(out, "\n")
}

func fallback(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
