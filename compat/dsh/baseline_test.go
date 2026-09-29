package dsh

import (
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
		Source       struct{ Version, Commit string }
		Distribution struct {
			Mode, Version, Integrity string
			SettingsPatch            struct{ BeforeSHA256, AfterSHA256 string }
		}
		State struct{ Workspace, Home, DshHome, Credentials string }
	}
	if err = json.Unmarshal(data, &baseline); err != nil {
		t.Fatal(err)
	}
	if baseline.Source.Version != "0.2.0-rc.2" || baseline.Source.Commit != "639ed015397290b3745d163aafe02ffee4aa3f84" {
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
	if len(baseline.Distribution.SettingsPatch.BeforeSHA256) != 64 || len(baseline.Distribution.SettingsPatch.AfterSHA256) != 64 {
		t.Fatal("settings patch must be exact")
	}
	if baseline.State.Workspace != "/var/lib/dsh/data/workspace" || baseline.State.Home != "/var/lib/dsh/data/home" || baseline.State.DshHome != "/var/lib/dsh/data/dsh" || baseline.State.Credentials != "$DSH_HOME/.credentials.yaml" {
		t.Fatal("single PVC path contract drift")
	}
}
