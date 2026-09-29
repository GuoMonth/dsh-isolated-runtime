package accesscontract

import (
	"strings"
	"testing"
)

func TestEnvoyOAuthCookieContract(t *testing.T) {
	t.Parallel()
	for _, name := range EnvoyOAuthCookieNames {
		if !IsEnvoyOAuthCookie(name) || !IsEnvoyOAuthCookie(strings.ToLower(name)) {
			t.Fatalf("reserved cookie %q was not recognized", name)
		}
		// Gateway v1.9.1 formats FNV-1a Sum32 with %x, without zero padding.
		for _, suffix := range []string{"0", "f", "aB", "abc", "1234", "abcde", "ABCDEF", "5f93c2e", "5f93C2e4"} {
			if !IsEnvoyOAuthCookie(name+"-"+suffix) || !IsEnvoyOAuthCookie(strings.ToLower(name)+"-"+suffix) {
				t.Fatalf("suffixed reserved cookie %q was not recognized", name+"-"+suffix)
			}
		}
	}
	for _, name := range []string{
		"dsh-auth-example", "theme", "session", "AccessToken-", "AccessToken-5f93c2e45",
		"AccessToken-nothex12", "myAccessToken-5f93c2e4", "AccessToken-preference",
	} {
		if IsEnvoyOAuthCookie(name) {
			t.Fatalf("application cookie %q was classified as OAuth state", name)
		}
	}
}
