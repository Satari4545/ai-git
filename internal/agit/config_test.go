package agit

import (
	"testing"

	gitwrap "github.com/Satari4545/ai-git/internal/git"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func agitConfigValue(t *testing.T, key string) string {
	t.Helper()

	value, err := gitwrap.OutputQuiet("config", "--get", key)
	if err != nil {
		t.Fatalf("git config --get %s failed: %v", key, err)
	}
	return value
}

func TestConfigCheckoutFeedRoundTrip(t *testing.T) {
	testutil.InitRepo(t)

	for _, tc := range []struct {
		arg      string
		stored   string
		modeWant string
	}{
		{"enable", "true", "enabled"},
		{"disable", "false", "disabled"},
		{"prompt", "prompt", "prompt"},
	} {
		if err := Config([]string{"checkout-feed", tc.arg}); err != nil {
			t.Fatalf("Config(checkout-feed %s) failed: %v", tc.arg, err)
		}
		if got := agitConfigValue(t, "agit.checkoutFeed"); got != tc.stored {
			t.Errorf("checkout-feed %s stored %q, want %q", tc.arg, got, tc.stored)
		}
		if got := CheckoutFeedMode(); got != tc.modeWant {
			t.Errorf("checkout-feed %s: mode = %q, want %q", tc.arg, got, tc.modeWant)
		}
	}
}

func TestConfigContextProviderRoundTrip(t *testing.T) {
	testutil.InitRepo(t)

	if err := Config([]string{"context-provider", "set", "my-provider --json"}); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if got := agitConfigValue(t, "agit.contextCommand"); got != "my-provider --json" {
		t.Errorf("stored provider = %q", got)
	}

	if err := Config([]string{"context-provider", "get"}); err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if err := Config([]string{"context-provider", "unset"}); err != nil {
		t.Fatalf("unset failed: %v", err)
	}
	if _, err := gitwrap.OutputQuiet("config", "--get", "agit.contextCommand"); err == nil {
		t.Error("expected agit.contextCommand to be unset")
	}
}

func TestConfigContextTimeout(t *testing.T) {
	testutil.InitRepo(t)

	if err := Config([]string{"context-timeout", "42"}); err != nil {
		t.Fatalf("Config(context-timeout) failed: %v", err)
	}
	if got := agitConfigValue(t, "agit.contextTimeoutSec"); got != "42" {
		t.Errorf("stored timeout = %q, want 42", got)
	}
}

func TestConfigUnknownArgsShowsUsage(t *testing.T) {
	testutil.InitRepo(t)

	if err := Config([]string{"bogus"}); err != nil {
		t.Errorf("expected nil error for unknown config args, got %v", err)
	}
	if err := Config(nil); err != nil {
		t.Errorf("expected nil error for empty config args, got %v", err)
	}
}
