package dsh

import (
	"bytes"
	"encoding/json"
	"os"
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
		wantVersion = "0.2.1-rc.5"
		wantCommit  = "b830ec950faba0a46dda564be3e2d854afed94f1"
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
		Packages map[string]struct{ Version, Integrity string }
	}
	if err = json.Unmarshal(lockData, &lock); err != nil {
		t.Fatal(err)
	}
	pkg := lock.Packages["node_modules/@deepseek-ai/dsh"]
	if pkg.Version != baseline.Source.Version || pkg.Integrity != baseline.Distribution.Integrity {
		t.Fatal("npm lock integrity drift")
	}
	if baseline.Distribution.SettingsPatch != nil {
		t.Fatal("0.2.1 persists settings natively; no settings patch is allowed")
	}
	if baseline.State.Workspace != "/var/lib/dsh/data/workspace" || baseline.State.Home != "/var/lib/dsh/data/home" || baseline.State.DshHome != "/var/lib/dsh/data/dsh" || baseline.State.Credentials != "$DSH_HOME/.credentials.yaml" {
		t.Fatal("single PVC path contract drift")
	}
	var manifest struct {
		Dependencies map[string]string `json:"dependencies"`
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
	if !manifest.AllowScripts["@deepseek-ai/dsh-subprocess-local@"+wantVersion] {
		t.Fatal("allowScripts must approve the locked subprocess-local install script")
	}
	dockerfile, err := os.ReadFile("../../images/environment/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(dockerfile, []byte(`io.dsh.isolated.dsh.version="`+wantVersion+`"`)) || !bytes.Contains(dockerfile, []byte(`io.dsh.isolated.contract.version="`+ContractVersion+`"`)) {
		t.Fatal("image label drift")
	}
}
