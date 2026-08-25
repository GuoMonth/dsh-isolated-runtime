package kubernetes

import (
	"testing"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

func TestPodForSpecHardensStandardRuntime(t *testing.T) {
	p := podForSpec("test", runtime.Spec{Name: "demo", Tenant: "tenant-a", Image: "nginxinc/nginx-unprivileged:alpine", SecurityClass: runtime.SecurityStandard}, "hash")
	if p.Spec.AutomountServiceAccountToken == nil || *p.Spec.AutomountServiceAccountToken {
		t.Fatal("service account token must be disabled")
	}
	sc := p.Spec.Containers[0].SecurityContext
	if sc.AllowPrivilegeEscalation || !sc.RunAsNonRoot {
		t.Fatalf("unexpected security context: %#v", sc)
	}
	if len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Fatalf("capabilities=%v", sc.Capabilities.Drop)
	}
	if sc.SeccompProfile.Type != "RuntimeDefault" {
		t.Fatalf("seccomp=%s", sc.SeccompProfile.Type)
	}
}

func TestSandboxedRequiresRuntimeClass(t *testing.T) {
	_, err := normalizeSpec(runtime.Spec{Name: "demo", Tenant: "tenant-a", Image: "image", SecurityClass: runtime.SecuritySandboxed})
	if err == nil {
		t.Fatal("expected sandboxed runtimeClass validation")
	}
}
