// Package dsh is the workload's record of the official DSH release.
// baseline.json is the pin embedded in the launcher. The compatibility test
// checks that file, the image label, the npm manifest and the lockfile.
package dsh

import (
	_ "embed"
	"encoding/json"
)

// ContractVersion is the single-PVC workload contract on the image label and
// the launcher /version response.
const ContractVersion = "dsh-021-single-pvc-v1"

//go:embed baseline.json
var baselineJSON []byte

func init() {
	var document struct {
		Source struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
		} `json:"source"`
	}
	if err := json.Unmarshal(baselineJSON, &document); err != nil {
		panic("compat/dsh/baseline.json: " + err.Error())
	}
	if document.Source.Version == "" || document.Source.Commit == "" {
		panic("compat/dsh/baseline.json: missing source pin")
	}
	version = document.Source.Version
	commit = document.Source.Commit
}

var (
	version string
	commit  string
)

// Version is the @deepseek-ai/dsh release recorded in baseline.json.
func Version() string { return version }

// Commit is the deepseek-harness commit recorded in baseline.json.
func Commit() string { return commit }
