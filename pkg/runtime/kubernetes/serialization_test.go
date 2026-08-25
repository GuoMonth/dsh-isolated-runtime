package kubernetes

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRuntimeCRSpecKeepsExplicitNetworkIsolationFalse(t *testing.T) {
	data, err := json.Marshal(runtimeCRSpec{Tenant: "tenant-a", Image: "image", NetworkIsolation: false})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"networkIsolation":false`) {
		t.Fatalf("networkIsolation=false was omitted: %s", data)
	}
}
