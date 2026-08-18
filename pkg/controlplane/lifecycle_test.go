package controlplane

import (
	"context"
	"errors"
	"testing"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime/kubernetes"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/scheduling"
)

func TestCreateSessionRejectsRuntimeTenantMismatch(t *testing.T) {
	l := New(kubernetes.New(), &scheduling.FirstFit{})

	_, err := l.CreateSession(context.Background(), CreateSessionRequest{
		Tenant:  "tenant-a",
		Session: "session-1",
		Runtime: runtime.Spec{Name: "runtime-1", Tenant: "tenant-b"},
	})
	if !errors.Is(err, ErrTenantOwnershipMismatch) {
		t.Fatalf("err = %v, want ErrTenantOwnershipMismatch", err)
	}
}
