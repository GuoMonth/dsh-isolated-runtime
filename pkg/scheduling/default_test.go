package scheduling

import (
	"context"
	"testing"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

func TestFirstFitNeverReusesForeignRuntime(t *testing.T) {
	a := &FirstFit{Runtimes: []runtime.Info{
		{Name: "foreign", Tenant: "tenant-b", Phase: "Running"},
	}}

	got, err := a.Allocate(context.Background(), Request{
		Tenant:             "tenant-a",
		Session:            "session-1",
		DesiredRuntimeName: "tenant-a-session-1",
		AllowReuse:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Reuse || got.RuntimeName != "tenant-a-session-1" {
		t.Fatalf("allocation = %#v, want new tenant-a runtime", got)
	}
}

func TestFirstFitMayReuseSameTenantRuntime(t *testing.T) {
	a := &FirstFit{Runtimes: []runtime.Info{
		{Name: "tenant-a-runtime", Tenant: "tenant-a", Phase: "Running"},
	}}

	got, err := a.Allocate(context.Background(), Request{
		Tenant:             "tenant-a",
		Session:            "session-1",
		DesiredRuntimeName: "tenant-a-session-1",
		AllowReuse:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Reuse || got.RuntimeName != "tenant-a-runtime" {
		t.Fatalf("allocation = %#v, want reuse tenant-a-runtime", got)
	}
}
