package gateway

import (
	"context"
	"errors"
	"testing"
)

type testAuthorizer struct {
	subject string
	tenant  string
}

func (a *testAuthorizer) Authorize(ctx context.Context, principal Principal, tenant string) error {
	a.subject = principal.Subject
	a.tenant = tenant
	return nil
}

type testResolver struct{}

func (testResolver) Resolve(ctx context.Context, tenant, session string) (string, error) {
	if tenant == "tenant-a" && session == "session-1" {
		return "runtime-1", nil
	}
	return "", errors.New("not found")
}

func TestAdmissionRequiresTrustedPrincipalContext(t *testing.T) {
	auth := &testAuthorizer{}
	admitter := NewAdmitter(auth, testResolver{})

	if _, err := admitter.Admit(context.Background(), AdmissionRequest{
		Tenant: "tenant-a", Session: "session-1",
	}); !errors.Is(err, ErrDenied) {
		t.Fatalf("without trusted principal err = %v, want ErrDenied", err)
	}

	ctx := WithPrincipal(context.Background(), Principal{Subject: "user-1"})
	got, err := admitter.Admit(ctx, AdmissionRequest{
		Tenant: "tenant-a", Session: "session-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Allowed || got.RuntimeRef != "runtime-1" {
		t.Fatalf("decision = %#v", got)
	}
	if auth.subject != "user-1" || auth.tenant != "tenant-a" {
		t.Fatalf("authorizer saw subject=%q tenant=%q", auth.subject, auth.tenant)
	}
}
