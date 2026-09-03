package dsh

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

//go:embed baseline.json
var baselineJSON []byte

type baseline struct {
	SchemaVersion int `json:"schemaVersion"`
	Source        struct {
		Repository string `json:"repository"`
		Tag        string `json:"tag"`
		Commit     string `json:"commit"`
		Version    string `json:"version"`
	} `json:"source"`
	Toolchain struct {
		PackageManager string `json:"packageManager"`
		LockfileSHA256 string `json:"lockfileSHA256"`
	} `json:"toolchain"`
	Distribution struct {
		Package   string `json:"package"`
		Version   string `json:"version"`
		Tarball   string `json:"tarball"`
		Integrity string `json:"integrity"`
	} `json:"distribution"`
	State []struct {
		Class    string `json:"class"`
		Location string `json:"location"`
		Snapshot bool   `json:"snapshot"`
	} `json:"state"`
	UpstreamTests []string `json:"upstreamTests"`
}

func TestBaselineIsExactAndComplete(t *testing.T) {
	var value baseline
	if err := json.Unmarshal(baselineJSON, &value); err != nil {
		t.Fatal(err)
	}
	if value.SchemaVersion != 1 || value.Source.Repository != "https://github.com/deepseek-ai/deepseek-harness.git" {
		t.Fatalf("unexpected source contract: %+v", value.Source)
	}
	if value.Source.Tag != "dsh-v0.1.2-rc.1" || value.Source.Version != "0.1.2-rc.1" {
		t.Fatalf("unexpected DSH release: %+v", value.Source)
	}
	if len(value.Source.Commit) != 40 {
		t.Fatalf("commit is not a full SHA-1: %q", value.Source.Commit)
	}
	if _, err := hex.DecodeString(value.Source.Commit); err != nil {
		t.Fatalf("invalid commit: %v", err)
	}
	if value.Toolchain.PackageManager != "pnpm@11.7.0" || len(value.Toolchain.LockfileSHA256) != 64 {
		t.Fatalf("toolchain is not exact: %+v", value.Toolchain)
	}
	if value.Distribution.Package != "@deepseek-ai/dsh" || value.Distribution.Version != value.Source.Version ||
		value.Distribution.Tarball != "https://registry.npmjs.org/@deepseek-ai/dsh/-/dsh-0.1.2-rc.1.tgz" ||
		value.Distribution.Integrity != "sha512-RPq48TzxvwpdT9/7W1tbhZDBMmeK+bxDrX9cqQC27Wx/LqtgJF8PSa3b3xriU8oxtvhwYmk21w2cej3uMQrnVA==" {
		t.Fatalf("runtime distribution is not exact: %+v", value.Distribution)
	}
	if strings.Contains(strings.ToLower(string(baselineJSON)), "latest") {
		t.Fatal("compatibility baseline contains a floating latest reference")
	}
	wantState := map[string]bool{
		"sessions": true, "attachments": true, "storage-domains": true,
		"workspace": true, "configuration": true, "provider-credentials": false,
		"browser-signing-records": false,
	}
	for _, item := range value.State {
		expected, ok := wantState[item.Class]
		if !ok || item.Location == "" || item.Snapshot != expected {
			t.Fatalf("unexpected state contract: %+v", item)
		}
		delete(wantState, item.Class)
	}
	if len(wantState) != 0 {
		t.Fatalf("missing state classes: %v", wantState)
	}
	if len(value.UpstreamTests) < 8 {
		t.Fatalf("compatibility suite is too small: %v", value.UpstreamTests)
	}
}
