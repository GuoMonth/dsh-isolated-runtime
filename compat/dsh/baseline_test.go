package dsh

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"strings"
	"testing"
)

func TestBaselineIsExactAndComplete(t *testing.T) {
	data, err := os.ReadFile("baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	var baseline struct {
		SchemaVersion int
		Source        struct{ Version, Commit string }
		Distribution  struct {
			Mode, Version, Integrity string
			SettingsPatch            *struct{ BeforeSHA256, AfterSHA256 string }
		}
		State struct{ Workspace, Home, DshHome, Credentials string }
	}
	if err = json.Unmarshal(data, &baseline); err != nil {
		t.Fatal(err)
	}
	if baseline.SchemaVersion != 3 {
		t.Fatal("baseline schema drift")
	}
	const (
		wantVersion = "0.2.2-rc.5"
		wantCommit  = "9c2670d3bce5275e0d1610272958121af87787bb"
	)
	if baseline.Source.Version != wantVersion || baseline.Source.Commit != wantCommit || Version() != wantVersion || Commit() != wantCommit {
		t.Fatal("DSH source pin drift")
	}
	if baseline.Distribution.Mode != "npm" || baseline.Distribution.Version != baseline.Source.Version {
		t.Fatal("use official exact npm release")
	}
	lockData, err := os.ReadFile("../../images/environment/package-lock.json")
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Packages map[string]struct {
			Version, Integrity string
			HasInstallScript   bool
		}
	}
	if err = json.Unmarshal(lockData, &lock); err != nil {
		t.Fatal(err)
	}
	pkg := lock.Packages["node_modules/@deepseek-ai/dsh"]
	if pkg.Version != baseline.Source.Version || pkg.Integrity != baseline.Distribution.Integrity {
		t.Fatal("npm lock integrity drift")
	}
	if baseline.Distribution.SettingsPatch != nil {
		t.Fatal("DSH persists settings natively; no settings patch is allowed")
	}
	if baseline.State.Workspace != "/var/lib/dsh/data/workspace" || baseline.State.Home != "/var/lib/dsh/data/home" || baseline.State.DshHome != "/var/lib/dsh/data/dsh" || baseline.State.Credentials != "$DSH_HOME/.credentials.yaml" {
		t.Fatal("single PVC path contract drift")
	}
	var manifest struct {
		Dependencies map[string]string `json:"dependencies"`
		Overrides    map[string]string `json:"overrides"`
		AllowScripts map[string]bool   `json:"allowScripts"`
	}
	manifestData, err := os.ReadFile("../../images/environment/package.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Dependencies["@deepseek-ai/dsh"] != wantVersion {
		t.Fatal("workload package.json pin drift")
	}
	lockedScripts := make(map[string]bool)
	for path, dependency := range lock.Packages {
		parts := strings.Split(path, "node_modules/")
		name := parts[len(parts)-1]
		if (name == "@deepseek-ai/dsh" || strings.HasPrefix(name, "@deepseek-ai/dsh-")) && dependency.Version != wantVersion {
			t.Errorf("official package closure differs from the DSH pin: %s at %s", path, dependency.Version)
		}
		if dependency.HasInstallScript {
			key := name + "@" + dependency.Version
			allowed, reviewed := manifest.AllowScripts[key]
			if !reviewed {
				t.Errorf("install script needs a review: %s", key)
			}
			lockedScripts[key] = allowed
		}
	}
	wantOverrides := map[string]string{
		"@modelcontextprotocol/client": "2.4.0",
		"fflate":                       "0.8.3",
		"undici":                       "8.12.4",
	}
	if !maps.Equal(manifest.Overrides, wantOverrides) {
		t.Fatal("only the reviewed SDK security overrides are allowed")
	}
	for name, version := range wantOverrides {
		if lock.Packages["node_modules/"+name].Version != version {
			t.Errorf("security override absent from the lockfile: %s", name)
		}
		for path, dependency := range lock.Packages {
			if strings.HasSuffix(path, "/node_modules/"+name) && dependency.Version != version {
				t.Errorf("unpatched nested SDK: %s at %s", path, dependency.Version)
			}
		}
	}
	wantScripts := map[string]bool{
		"@deepseek-ai/dsh-subprocess-local@" + wantVersion: true,
		"@google/genai@2.32.0":                             false,
		"koffi@3.1.1":                                      true,
		"node-pty@1.2.0-beta.15":                           true,
		"protobufjs@7.6.6":                                 false,
	}
	if !maps.Equal(manifest.AllowScripts, wantScripts) || !maps.Equal(lockedScripts, wantScripts) {
		t.Fatal("allowScripts must match the reviewed install scripts in the lockfile")
	}
	dockerfile, err := os.ReadFile("../../images/environment/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(dockerfile, []byte(`io.dsh.isolated.dsh.version="`+wantVersion+`"`)) || !bytes.Contains(dockerfile, []byte(`io.dsh.isolated.contract.version="`+ContractVersion+`"`)) {
		t.Fatal("image label drift")
	}
}
