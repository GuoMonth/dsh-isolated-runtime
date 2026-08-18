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
		SecurityClass:      runtime.SecuritySandboxed,
		Image:              "dsh-runtime:data",
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
		{
			Name:          "tenant-a-runtime",
			Tenant:        "tenant-a",
			SecurityClass: runtime.SecuritySandboxed,
			Image:         "dsh-runtime:data",
			Phase:         "Running",
		},
	}}

	got, err := a.Allocate(context.Background(), Request{
		Tenant:             "tenant-a",
		Session:            "session-1",
		DesiredRuntimeName: "tenant-a-session-1",
		AllowReuse:         true,
		SecurityClass:      runtime.SecuritySandboxed,
		Image:              "dsh-runtime:data",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Reuse || got.RuntimeName != "tenant-a-runtime" {
		t.Fatalf("allocation = %#v, want reuse tenant-a-runtime", got)
	}
}

func TestFirstFitDoesNotReuseWeakerOrDifferentProfile(t *testing.T) {
	a := &FirstFit{Runtimes: []runtime.Info{
		{
			Name:          "tenant-a-standard",
			Tenant:        "tenant-a",
			SecurityClass: runtime.SecurityStandard,
			Image:         "dsh-runtime:base",
			Phase:         "Running",
		},
	}}

	got, err := a.Allocate(context.Background(), Request{
		Tenant:             "tenant-a",
		Session:            "session-2",
		DesiredRuntimeName: "tenant-a-data",
		AllowReuse:         true,
		SecurityClass:      runtime.SecuritySandboxed,
		Image:              "dsh-runtime:data",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Reuse || got.RuntimeName != "tenant-a-data" {
		t.Fatalf("allocation = %#v, want new sandboxed data runtime", got)
	}
}
