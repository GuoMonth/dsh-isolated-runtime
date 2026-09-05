package accesscontract

import (
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	t.Parallel()
	disabled := Config{}
	if err := disabled.Validate(); err != nil || disabled.Enabled() {
		t.Fatalf("disabled config = enabled %v, error %v", disabled.Enabled(), err)
	}
	config := Config{
		GatewayName:       "dsh",
		GatewayNamespace:  "dsh-system",
		GatewaySection:    "https",
		BaseDomain:        "cells.test",
		ExternalHTTPSPort: 18443,
	}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	uid := "12345678-1234-1234-1234-123456789abc"
	if got, want := config.Hostname(uid), "cell-12345678-1234-1234-1234-123456789abc.cells.test"; got != want {
		t.Fatalf("hostname = %q, want %q", got, want)
	}
	if got, want := config.Authority(uid), "cell-12345678-1234-1234-1234-123456789abc.cells.test:18443"; got != want {
		t.Fatalf("authority = %q, want %q", got, want)
	}
	config.ExternalHTTPSPort = 443
	if got := config.Authority(uid); got != config.Hostname(uid) {
		t.Fatalf("default HTTPS authority = %q", got)
	}
}

func TestConfigRejectsPartialAndInvalidValues(t *testing.T) {
	t.Parallel()
	for name, config := range map[string]Config{
		"domain without gateway": {BaseDomain: "cells.test"},
		"missing domain":         {GatewayName: "dsh", GatewayNamespace: "dsh-system", GatewaySection: "https"},
		"bad domain":             {GatewayName: "dsh", GatewayNamespace: "dsh-system", GatewaySection: "https", BaseDomain: "Cells.Test"},
		"bad port":               {GatewayName: "dsh", GatewayNamespace: "dsh-system", GatewaySection: "https", BaseDomain: "cells.test", ExternalHTTPSPort: 70000},
		"derived host too long":  {GatewayName: "dsh", GatewayNamespace: "dsh-system", GatewaySection: "https", BaseDomain: strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 60)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := config.Validate(); err == nil {
				t.Fatal("invalid config was accepted")
			}
		})
	}
}

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
