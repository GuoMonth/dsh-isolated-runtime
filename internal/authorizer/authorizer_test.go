package authorizer

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/accesscontract"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

const (
	testIssuer = "https://idp.cells.test/dex"
	testUID    = types.UID("12345678-1234-1234-1234-123456789abc")
)

type fakeVerifier struct {
	identity Identity
	err      error
}

func (f fakeVerifier) Verify(context.Context, string) (Identity, error) {
	return f.identity, f.err
}

type fakeReviewer struct {
	allowed         bool
	err             error
	evaluationError string
	nilResult       bool
	last            *authorizationv1.SubjectAccessReview
}

func (f *fakeReviewer) Create(_ context.Context, review *authorizationv1.SubjectAccessReview, _ metav1.CreateOptions) (*authorizationv1.SubjectAccessReview, error) {
	f.last = review.DeepCopy()
	if f.err != nil {
		return nil, f.err
	}
	if f.nilResult {
		return nil, nil
	}
	return &authorizationv1.SubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{
		Allowed: f.allowed, EvaluationError: f.evaluationError,
	}}, nil
}

type failingReader struct{ client.Reader }

func (f failingReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return apierrors.NewInternalError(errors.New("api unavailable"))
}

type decisionRecorder struct {
	values []Decision
}

func (r *decisionRecorder) RecordDecision(decision Decision) {
	r.values = append(r.values, decision)
}

func TestCheckAllowsExactRouteAndBuildsSubjectAccessReview(t *testing.T) {
	t.Parallel()
	server, reviewer, request := testServer(t)
	response, err := server.Check(context.Background(), request)
	if err != nil || response.GetStatus().GetCode() != int32(codes.OK) {
		t.Fatalf("Check = %#v, %v", response, err)
	}
	attributes := reviewer.last.Spec.ResourceAttributes
	if reviewer.last.Spec.User != testIssuer+"#alice" || len(reviewer.last.Spec.Groups) != 2 || reviewer.last.Spec.Groups[1] != testIssuer+"#developers" {
		t.Fatalf("identity mapping = %#v", reviewer.last.Spec)
	}
	if attributes.Namespace != "tenant-a" || attributes.Name != "main" || attributes.Verb != "access" || attributes.Resource != "cells" {
		t.Fatalf("resource attributes = %#v", attributes)
	}
	if len(response.GetOkResponse().GetHeaders()) != 0 {
		t.Fatalf("authorizer injected headers: %#v", response.GetOkResponse().GetHeaders())
	}
}

func TestCheckRecordsClosedDecisionClasses(t *testing.T) {
	t.Parallel()
	server, reviewer, request := testServer(t)
	recorder := &decisionRecorder{}
	server.Decisions = recorder
	if _, err := server.Check(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	reviewer.allowed = false
	if _, err := server.Check(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	delete(request.Attributes.Request.Http.Headers, "x-dsh-oidc-token")
	if _, err := server.Check(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	want := []Decision{DecisionAllow, DecisionDenied, DecisionUnauthenticated}
	if len(recorder.values) != len(want) {
		t.Fatalf("decisions = %v, want %v", recorder.values, want)
	}
	for index := range want {
		if recorder.values[index] != want[index] {
			t.Fatalf("decisions = %v, want %v", recorder.values, want)
		}
	}
}

func TestMetricsUseOnlyClosedTopologyFreeLabels(t *testing.T) {
	t.Parallel()
	registry, metrics := NewMetricsRegistry()
	for _, decision := range allDecisions {
		metrics.RecordDecision(decision)
	}
	metrics.RecordDecision(Decision("tenant-a/token-value/cell-uid"))

	request := httptest.NewRequest("GET", "http://metrics/metrics", nil)
	response := httptest.NewRecorder()
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != 200 {
		t.Fatalf("metrics status = %d: %s", response.Code, body)
	}
	for _, forbidden := range []string{"tenant-a", "token-value", "cell-uid", "namespace=", "subject=", "route=", "image="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("metrics exposed forbidden value %q:\n%s", forbidden, body)
		}
	}
	for _, decision := range allDecisions {
		if !strings.Contains(body, `decision="`+string(decision)+`"`) {
			t.Fatalf("metrics omitted decision %q:\n%s", decision, body)
		}
	}
}

func TestCheckFailsClosed(t *testing.T) {
	t.Parallel()
	for name, scenario := range map[string]struct {
		mutate    func(*Server, *fakeReviewer, *authv3.CheckRequest)
		wantCode  int32
		wantError codes.Code
	}{
		"missing token": {
			mutate: func(_ *Server, _ *fakeReviewer, request *authv3.CheckRequest) {
				delete(request.Attributes.Request.Http.Headers, "x-dsh-oidc-token")
			},
			wantCode: int32(codes.Unauthenticated),
		},
		"invalid token": {
			mutate: func(server *Server, _ *fakeReviewer, _ *authv3.CheckRequest) {
				server.Verifier = fakeVerifier{err: errUnauthenticated}
			},
			wantCode: int32(codes.Unauthenticated),
		},
		"non https transport": {
			mutate: func(_ *Server, _ *fakeReviewer, request *authv3.CheckRequest) {
				request.Attributes.Request.Http.Scheme = "http"
			},
			wantCode: int32(codes.PermissionDenied),
		},
		"wrong host": {
			mutate: func(_ *Server, _ *fakeReviewer, request *authv3.CheckRequest) {
				request.Attributes.Request.Http.Host = "cell-other.cells.test"
			},
			wantCode: int32(codes.PermissionDenied),
		},
		"missing route metadata": {
			mutate: func(_ *Server, _ *fakeReviewer, request *authv3.CheckRequest) {
				request.Attributes.RouteMetadataContext = nil
			},
			wantCode: int32(codes.PermissionDenied),
		},
		"ambiguous route metadata": {
			mutate: func(_ *Server, _ *fakeReviewer, request *authv3.CheckRequest) {
				resources := request.Attributes.RouteMetadataContext.FilterMetadata[metadataKey].Fields["resources"].GetListValue()
				resources.Values = append(resources.Values, resources.Values[0])
			},
			wantCode: int32(codes.PermissionDenied),
		},
		"wrong route uid": {
			mutate: func(_ *Server, _ *fakeReviewer, request *authv3.CheckRequest) {
				root := request.Attributes.RouteMetadataContext.FilterMetadata[metadataKey]
				root.Fields["resources"].GetListValue().Values[0].GetStructValue().Fields["annotations"].GetStructValue().Fields["dsh-cell-uid"] = structpb.NewStringValue("other")
			},
			wantCode: int32(codes.PermissionDenied),
		},
		"route hostname drift": {
			mutate: func(server *Server, _ *fakeReviewer, _ *authv3.CheckRequest) {
				mutateRoute(server, func(route *gatewayv1.HTTPRoute) {
					route.Spec.Hostnames[0] = "other.cells.test"
				})
			},
			wantCode: int32(codes.PermissionDenied),
		},
		"route parent drift": {
			mutate: func(server *Server, _ *fakeReviewer, _ *authv3.CheckRequest) {
				mutateRoute(server, func(route *gatewayv1.HTTPRoute) {
					route.Spec.ParentRefs[0].Name = "other"
				})
			},
			wantCode: int32(codes.PermissionDenied),
		},
		"route backend drift": {
			mutate: func(server *Server, _ *fakeReviewer, _ *authv3.CheckRequest) {
				mutateRoute(server, func(route *gatewayv1.HTTPRoute) {
					route.Spec.Rules[0].BackendRefs[0].Name = "other"
				})
			},
			wantCode: int32(codes.PermissionDenied),
		},
		"route ownership drift": {
			mutate: func(server *Server, _ *fakeReviewer, _ *authv3.CheckRequest) {
				mutateRoute(server, func(route *gatewayv1.HTTPRoute) {
					route.OwnerReferences = nil
				})
			},
			wantCode: int32(codes.PermissionDenied),
		},
		"route read unavailable": {
			mutate: func(server *Server, _ *fakeReviewer, _ *authv3.CheckRequest) {
				server.Reader = failingReader{Reader: server.Reader}
			},
			wantError: codes.Unavailable,
		},
		"rbac deny": {
			mutate:   func(_ *Server, reviewer *fakeReviewer, _ *authv3.CheckRequest) { reviewer.allowed = false },
			wantCode: int32(codes.PermissionDenied),
		},
		"rbac unavailable": {
			mutate:    func(_ *Server, reviewer *fakeReviewer, _ *authv3.CheckRequest) { reviewer.err = errors.New("api down") },
			wantError: codes.Unavailable,
		},
		"rbac evaluation error": {
			mutate: func(_ *Server, reviewer *fakeReviewer, _ *authv3.CheckRequest) {
				reviewer.evaluationError = "authorizer unavailable"
			},
			wantError: codes.Unavailable,
		},
		"rbac empty response": {
			mutate: func(_ *Server, reviewer *fakeReviewer, _ *authv3.CheckRequest) {
				reviewer.nilResult = true
			},
			wantError: codes.Unavailable,
		},
		"jwks unavailable": {
			mutate: func(server *Server, _ *fakeReviewer, _ *authv3.CheckRequest) {
				server.Verifier = fakeVerifier{err: errUnavailable}
			},
			wantError: codes.Unavailable,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server, reviewer, request := testServer(t)
			scenario.mutate(server, reviewer, request)
			response, err := server.Check(context.Background(), request)
			if scenario.wantError != codes.OK {
				if status.Code(err) != scenario.wantError || response != nil {
					t.Fatalf("Check = %#v, %v; want gRPC %s", response, err, scenario.wantError)
				}
				return
			}
			if err != nil || response.GetStatus().GetCode() != scenario.wantCode {
				t.Fatalf("Check = %#v, %v; want status %d", response, err, scenario.wantCode)
			}
		})
	}
}

func TestOIDCVerifierChecksSignatureAudienceExpiryAndGroups(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, nil)
	if err != nil {
		t.Fatal(err)
	}
	keySet := &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}
	verifier := &OIDCVerifier{
		verifier:    oidc.NewVerifier(testIssuer, keySet, &oidc.Config{ClientID: "dsh-browser"}),
		groupsClaim: "groups",
	}
	now := time.Now()
	raw, err := jwt.Signed(signer).Claims(map[string]any{
		"iss":    testIssuer,
		"sub":    "alice",
		"aud":    "dsh-browser",
		"exp":    now.Add(time.Hour).Unix(),
		"iat":    now.Unix(),
		"groups": []string{"developers"},
	}).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	identity, err := verifier.Verify(context.Background(), raw)
	if err != nil || identity.Subject != "alice" || len(identity.Groups) != 1 || identity.Groups[0] != "developers" {
		t.Fatalf("Verify = %#v, %v", identity, err)
	}
	wrongAudience := *verifier
	wrongAudience.verifier = oidc.NewVerifier(testIssuer, keySet, &oidc.Config{ClientID: "other"})
	if _, err := wrongAudience.Verify(context.Background(), raw); err == nil {
		t.Fatal("wrong audience was accepted")
	}

	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	wrongSigner, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: wrongKey}, nil)
	if err != nil {
		t.Fatal(err)
	}
	wrongSignature, err := jwt.Signed(wrongSigner).Claims(map[string]any{
		"iss": testIssuer, "sub": "alice", "aud": "dsh-browser", "exp": now.Add(time.Hour).Unix(),
	}).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(context.Background(), wrongSignature); err == nil {
		t.Fatal("wrong signature was accepted")
	}

	for name, claims := range map[string]map[string]any{
		"expired": {
			"iss": testIssuer, "sub": "alice", "aud": "dsh-browser", "exp": now.Add(-time.Hour).Unix(),
		},
		"missing subject": {
			"iss": testIssuer, "aud": "dsh-browser", "exp": now.Add(time.Hour).Unix(),
		},
		"scalar groups": {
			"iss": testIssuer, "sub": "alice", "aud": "dsh-browser", "exp": now.Add(time.Hour).Unix(), "groups": "developers",
		},
		"non string group": {
			"iss": testIssuer, "sub": "alice", "aud": "dsh-browser", "exp": now.Add(time.Hour).Unix(), "groups": []any{"developers", 7},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			candidate, err := jwt.Signed(signer).Claims(claims).Serialize()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := verifier.Verify(context.Background(), candidate); err == nil {
				t.Fatal("invalid token was accepted")
			}
		})
	}
}

func mutateRoute(server *Server, mutate func(*gatewayv1.HTTPRoute)) {
	kube, ok := server.Reader.(client.Client)
	if !ok {
		panic("test reader is not writable")
	}
	names := cellcontract.ResourceNames(string(testUID))
	var route gatewayv1.HTTPRoute
	if err := kube.Get(context.Background(), client.ObjectKey{Namespace: "tenant-a", Name: names.Base}, &route); err != nil {
		panic(err)
	}
	mutate(&route)
	if err := kube.Update(context.Background(), &route); err != nil {
		panic(err)
	}
}

func testServer(t *testing.T) (*Server, *fakeReviewer, *authv3.CheckRequest) {
	t.Helper()
	config := accesscontract.Config{GatewayName: "dsh", GatewayNamespace: "dsh-system", GatewaySection: "https", BaseDomain: "cells.test"}
	cell := &dshv1alpha1.Cell{
		TypeMeta:   metav1.TypeMeta{APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "Cell"},
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "tenant-a", UID: testUID},
	}
	names := cellcontract.ResourceNames(string(cell.UID))
	group := gatewayv1.Group(gatewayv1.GroupVersion.Group)
	kind := gatewayv1.Kind("Gateway")
	namespace := gatewayv1.Namespace(config.GatewayNamespace)
	section := gatewayv1.SectionName(config.GatewaySection)
	pathType := gatewayv1.PathMatchPathPrefix
	root := "/"
	port := gatewayv1.PortNumber(80)
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.Base, Namespace: cell.Namespace,
			Annotations:     map[string]string{cellcontract.RouteCellNameAnnotation: cell.Name, cellcontract.RouteCellUIDAnnotation: string(cell.UID)},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "Cell", Name: cell.Name, UID: cell.UID, Controller: boolPointer(true)}},
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: []gatewayv1.ParentReference{{Group: &group, Kind: &kind, Namespace: &namespace, Name: gatewayv1.ObjectName(config.GatewayName), SectionName: &section}}},
			Hostnames:       []gatewayv1.Hostname{gatewayv1.Hostname(config.Hostname(string(cell.UID)))},
			Rules:           []gatewayv1.HTTPRouteRule{{Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Type: &pathType, Value: &root}}}, BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: gatewayv1.ObjectName(names.Base), Port: &port}}}}}},
		},
	}
	scheme := runtime.NewScheme()
	if err := dshv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := gatewayv1.Install(scheme); err != nil {
		t.Fatal(err)
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cell, route).Build()
	reviewer := &fakeReviewer{allowed: true}
	server := &Server{Reader: reader, Reviewer: reviewer, Verifier: fakeVerifier{identity: Identity{Subject: "alice", Groups: []string{"developers"}}}, Issuer: testIssuer, RouteConfig: config}
	metadata, err := structpb.NewStruct(map[string]any{
		"resources": []any{map[string]any{
			"kind": "HTTPRoute", "namespace": cell.Namespace, "name": names.Base,
			"annotations": map[string]any{"dsh-cell-name": cell.Name, "dsh-cell-uid": string(cell.UID)},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := &authv3.CheckRequest{Attributes: &authv3.AttributeContext{
		Request: &authv3.AttributeContext_Request{Http: &authv3.AttributeContext_HttpRequest{
			Scheme: "https", Host: config.Authority(string(cell.UID)), Headers: map[string]string{"x-dsh-oidc-token": "redacted.jwt"},
		}},
		RouteMetadataContext: &corev3.Metadata{FilterMetadata: map[string]*structpb.Struct{metadataKey: metadata}},
	}}
	return server, reviewer, request
}

func boolPointer(value bool) *bool { return &value }
