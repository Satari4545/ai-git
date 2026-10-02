package agit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func writeFakeCLI(t *testing.T, dir, name, body string) {
	t.Helper()
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func gitOnlyDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git not on PATH: %v", err)
	}
	if err := os.Symlink(gitPath, filepath.Join(dir, "git")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBridgeExecutableRegistered(t *testing.T) {
	for _, name := range BridgeNames() {
		exe, err := BridgeExecutable(name)
		if err != nil {
			t.Errorf("BridgeExecutable(%q) = %v, want nil", name, err)
			continue
		}
		if exe == "" {
			t.Errorf("BridgeExecutable(%q) = empty, want non-empty", name)
		}
	}
}

func TestBridgeExecutableUnknown(t *testing.T) {
	if _, err := BridgeExecutable("no-such-bridge"); err == nil {
		t.Error("BridgeExecutable(unknown) = nil error, want error")
	}
}

func TestMCPInstallTargets(t *testing.T) {
	want := []string{"codex", "claude", "pi", "cursor", "gemini"}
	got := MCPInstallTargets()
	if len(got) != len(want) {
		t.Fatalf("MCPInstallTargets() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MCPInstallTargets() = %v, want %v", got, want)
		}
	}
}

func setupHealthyDoctorRepo(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "remote", "add", "origin", "https://example.com/repo.git")
	if err := Init(); err != nil {
		t.Fatalf("Init() = %v, want nil", err)
	}
	testutil.RunGit(t, dir, "config", "agit.bridge", "codex")

	bin := t.TempDir()
	writeFakeCLI(t, bin, "codex", "exit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	configPath, err := codexMCPConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("[mcp_servers.agit]\ncommand = \"agit\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	testutil.RunGit(t, dir, "commit", "--allow-empty", "-m", "test")
	sha := strings.TrimSpace(testutil.RunGit(t, dir, "rev-parse", "HEAD"))
	testutil.RunGit(t, dir, "update-ref", NotesRef, sha)

	stateDir := filepath.Join(dir, ".agit", "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	checkoutJSON := `{"schemaVersion":"1","commit":"abc","restoredAt":"now","note":{}}`
	if err := os.WriteFile(filepath.Join(stateDir, "checkout.json"), []byte(checkoutJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDoctorHealthyRepo(t *testing.T) {
	setupHealthyDoctorRepo(t)

	out := captureStdout(t, func() {
		if err := Doctor(); err != nil {
			t.Fatalf("Doctor() = %v, want nil", err)
		}
	})

	for _, want := range []string{
		"[PASS] git repository",
		"[PASS] agit init (notes refspec)",
		"[PASS] bridge configured",
		"[PASS] bridge CLI on PATH",
		"[PASS] MCP server installed",
		"[PASS] local agit notes",
		"[PASS] restored checkout context",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "[WARN]") || strings.Contains(out, "[FAIL]") {
		t.Errorf("healthy repo produced WARN/FAIL:\n%s", out)
	}
}

func TestDoctorNotARepo(t *testing.T) {
	dir := t.TempDir()
	chdirTo(t, dir)

	out := captureStdout(t, func() {
		if err := Doctor(); err == nil {
			t.Fatal("Doctor() = nil outside a repo, want error")
		}
	})
	if !strings.Contains(out, "[FAIL] git repository") {
		t.Errorf("doctor output missing [FAIL] git repository:\n%s", out)
	}
}

func TestDoctorMissingRefspecWarns(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "remote", "add", "origin", "https://example.com/repo.git")
	testutil.RunGit(t, dir, "config", "agit.bridge", "codex")

	bin := t.TempDir()
	writeFakeCLI(t, bin, "codex", "exit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	out := captureStdout(t, func() {
		if err := Doctor(); err != nil {
			t.Fatalf("Doctor() = %v, want nil (WARNs only)", err)
		}
	})
	for _, want := range []string{
		"[WARN] agit init (notes refspec)",
		"run `agit init`",
		"[WARN] MCP server installed",
		"agit mcp install codex",
		"[WARN] local agit notes",
		"[WARN] restored checkout context",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output missing %q\n%s", want, out)
		}
	}
}

func TestDoctorBridgeCLIMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", gitOnlyDir(t))

	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "config", "agit.bridge", "codex")

	out := captureStdout(t, func() {
		if err := Doctor(); err != nil {
			t.Fatalf("Doctor() = %v, want nil (WARNs only)", err)
		}
	})
	if !strings.Contains(out, "[WARN] bridge CLI on PATH") {
		t.Errorf("doctor output missing [WARN] bridge CLI on PATH:\n%s", out)
	}
	if !strings.Contains(out, "not found on PATH") {
		t.Errorf("doctor output missing hint about missing CLI:\n%s", out)
	}
}

func TestDoctorInvalidBridge(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "config", "agit.bridge", "bogus")

	out := captureStdout(t, func() {
		if err := Doctor(); err == nil {
			t.Fatal("Doctor() = nil with invalid agit.bridge, want error")
		}
	})
	if !strings.Contains(out, "[FAIL] bridge configured") {
		t.Errorf("doctor output missing [FAIL] bridge configured:\n%s", out)
	}
	if !strings.Contains(out, "invalid agit.bridge") {
		t.Errorf("doctor output missing invalid agit.bridge hint:\n%s", out)
	}
}

func TestDoctorClaudeMCPViaCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "config", "agit.bridge", "claude-code")

	bin := t.TempDir()
	writeFakeCLI(t, bin, "claude", "echo 'agit: agit mcp serve (stdio)'\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	out := captureStdout(t, func() {
		if err := Doctor(); err != nil {
			t.Fatalf("Doctor() = %v, want nil", err)
		}
	})
	if !strings.Contains(out, "[PASS] MCP server installed") {
		t.Errorf("doctor output missing [PASS] MCP server installed:\n%s", out)
	}
}

func TestInitSuggestsDetectedBridge(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	testutil.InitRepo(t)

	bin := t.TempDir()
	writeFakeCLI(t, bin, "gemini", "exit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+gitOnlyDir(t))

	out := captureStdout(t, func() {
		if err := Init(); err != nil {
			t.Fatalf("Init() = %v, want nil", err)
		}
	})
	if !strings.Contains(out, "agit mcp install gemini") {
		t.Errorf("init output missing detected bridge suggestion:\n%s", out)
	}
}

func TestInitSuggestsMultipleDetectedBridges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	testutil.InitRepo(t)

	bin := t.TempDir()
	writeFakeCLI(t, bin, "codex", "exit 0\n")
	writeFakeCLI(t, bin, "gemini", "exit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+gitOnlyDir(t))

	out := captureStdout(t, func() {
		if err := Init(); err != nil {
			t.Fatalf("Init() = %v, want nil", err)
		}
	})
	if !strings.Contains(out, "detected on PATH: codex, gemini") {
		t.Errorf("init output missing multi-bridge suggestion:\n%s", out)
	}
}

func TestInitSuggestsGenericWhenNoneDetected(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", gitOnlyDir(t))
	testutil.InitRepo(t)

	out := captureStdout(t, func() {
		if err := Init(); err != nil {
			t.Fatalf("Init() = %v, want nil", err)
		}
	})
	if !strings.Contains(out, "agit mcp install <agent>") {
		t.Errorf("init output missing generic suggestion:\n%s", out)
	}
}
