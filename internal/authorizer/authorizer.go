// Package authorizer implements the narrow Envoy ext_authz to Kubernetes RBAC
// seam used by Phase 2. It never proxies Cell traffic.
package authorizer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/accesscontract"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

const metadataKey = "envoy-gateway"

var (
	errUnauthenticated = errors.New("identity is missing or invalid")
	errForbidden       = errors.New("cell access is forbidden")
	errUnavailable     = errors.New("authorization dependency is unavailable")
)

type Identity struct {
	Subject string
	Groups  []string
}

type TokenVerifier interface {
	Verify(context.Context, string) (Identity, error)
}

type Reviewer interface {
	Create(context.Context, *authorizationv1.SubjectAccessReview, metav1.CreateOptions) (*authorizationv1.SubjectAccessReview, error)
}

// Decision is a closed, topology- and identity-free authorization outcome.
// Values are also metric label values, so callers must never derive one from
// an error, claim, route, or Kubernetes object.
type Decision string

const (
	DecisionAllow           Decision = "allow"
	DecisionUnauthenticated Decision = "unauthenticated"
	DecisionDenied          Decision = "denied"
	DecisionRouteMismatch   Decision = "route_mismatch"
	DecisionDependencyError Decision = "dependency_error"
)

var allDecisions = [...]Decision{
	DecisionAllow,
	DecisionUnauthenticated,
	DecisionDenied,
	DecisionRouteMismatch,
	DecisionDependencyError,
}

type DecisionRecorder interface {
	RecordDecision(Decision)
}

type Server struct {
	authv3.UnimplementedAuthorizationServer

	Reader      client.Reader
	Reviewer    Reviewer
	Verifier    TokenVerifier
	Issuer      string
	RouteConfig accesscontract.Config
	Decisions   DecisionRecorder
}

func (s *Server) Validate() error {
	if s.Reader == nil || s.Reviewer == nil || s.Verifier == nil {
		return errors.New("authorizer dependencies are required")
	}
	if strings.TrimSpace(s.Issuer) == "" {
		return errors.New("OIDC issuer is required")
	}
	if !s.RouteConfig.Enabled() {
		return errors.New("gateway routing must be enabled for the authorizer")
	}
	return s.RouteConfig.Validate()
}

func (s *Server) Check(ctx context.Context, request *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	if err := s.authorize(ctx, request); err != nil {
		s.recordDecision(decisionForError(err))
		// Decision reasons are deliberately coarse: they make fail-closed behavior
		// diagnosable without logging tokens, claims, authorities, or route data.
		log.Printf("cell-authorizer decision=deny reason=%s", decisionReason(err))
		switch {
		case errors.Is(err, errUnauthenticated):
			return denied(typev3.StatusCode_Unauthorized, codes.Unauthenticated, "unauthorized"), nil
		case errors.Is(err, errForbidden):
			return denied(typev3.StatusCode_Forbidden, codes.PermissionDenied, "forbidden"), nil
		default:
			return nil, status.Error(codes.Unavailable, "authorization unavailable")
		}
	}
	s.recordDecision(DecisionAllow)
	return &authv3.CheckResponse{
		Status: &statuspb.Status{Code: int32(codes.OK)},
		HttpResponse: &authv3.CheckResponse_OkResponse{
			OkResponse: &authv3.OkHttpResponse{},
		},
	}, nil
}

func (s *Server) recordDecision(decision Decision) {
	if s.Decisions != nil {
		s.Decisions.RecordDecision(decision)
	}
}

func decisionForError(err error) Decision {
	switch {
	case errors.Is(err, errUnauthenticated):
		return DecisionUnauthenticated
	case errors.Is(err, errUnavailable):
		return DecisionDependencyError
	case errors.Is(err, errRBACDenied):
		return DecisionDenied
	default:
		return DecisionRouteMismatch
	}
}

func decisionReason(err error) string {
	message := err.Error()
	if index := strings.LastIndexByte(message, ':'); index >= 0 {
		return strings.TrimSpace(message[index+1:])
	}
	return "unknown"
}

func denied(httpStatus typev3.StatusCode, grpcCode codes.Code, body string) *authv3.CheckResponse {
	return &authv3.CheckResponse{
		Status: &statuspb.Status{Code: int32(grpcCode)},
		HttpResponse: &authv3.CheckResponse_DeniedResponse{DeniedResponse: &authv3.DeniedHttpResponse{
			Status: &typev3.HttpStatus{Code: httpStatus},
			Body:   body,
		}},
	}
}

func (s *Server) authorize(ctx context.Context, request *authv3.CheckRequest) error {
	attributes := request.GetAttributes()
	httpRequest := attributes.GetRequest().GetHttp()
	if httpRequest == nil || !strings.EqualFold(httpRequest.GetScheme(), "https") {
		return fmt.Errorf("%w: transport", errForbidden)
	}
	rawToken := strings.TrimSpace(httpRequest.GetHeaders()[strings.ToLower(cellcontract.OIDCTokenHeader)])
	if rawToken == "" {
		return fmt.Errorf("%w: identity-missing", errUnauthenticated)
	}
	identity, err := s.Verifier.Verify(ctx, rawToken)
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return fmt.Errorf("%w: oidc-dependency", errUnavailable)
		}
		return fmt.Errorf("%w: identity-invalid", errUnauthenticated)
	}

	info, err := routeFromMetadata(attributes.GetRouteMetadataContext())
	if err != nil {
		return fmt.Errorf("%w: route-metadata", errForbidden)
	}
	cell, err := s.validateRoute(ctx, info, httpRequest.GetHost())
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return fmt.Errorf("%w: kubernetes-read", errUnavailable)
		}
		return fmt.Errorf("%w: route-contract", errForbidden)
	}

	groups := make([]string, 0, len(identity.Groups)+1)
	groups = append(groups, "system:authenticated")
	for _, group := range identity.Groups {
		groups = append(groups, s.Issuer+"#"+group)
	}
	review := &authorizationv1.SubjectAccessReview{Spec: authorizationv1.SubjectAccessReviewSpec{
		User:   s.Issuer + "#" + identity.Subject,
		Groups: groups,
		ResourceAttributes: &authorizationv1.ResourceAttributes{
			Namespace: cell.Namespace,
			Verb:      "access",
			Group:     dshv1alpha1.GroupVersion.Group,
			Version:   dshv1alpha1.GroupVersion.Version,
			Resource:  "cells",
			Name:      cell.Name,
		},
	}}
	result, err := s.Reviewer.Create(ctx, review, metav1.CreateOptions{})
	if err != nil || result == nil || result.Status.EvaluationError != "" {
		return fmt.Errorf("%w: kubernetes-review", errUnavailable)
	}
	if !result.Status.Allowed {
		return fmt.Errorf("%w: %w", errForbidden, errRBACDenied)
	}
	return nil
}

type routeInfo struct {
	Namespace string
	Name      string
	CellName  string
	CellUID   string
}

func routeFromMetadata(metadata *corev3.Metadata) (routeInfo, error) {
	if metadata == nil {
		return routeInfo{}, errors.New("route metadata is missing")
	}
	root := metadata.GetFilterMetadata()[metadataKey]
	resources := root.GetFields()["resources"].GetListValue().GetValues()
	var found []routeInfo
	for _, resource := range resources {
		fields := resource.GetStructValue().GetFields()
		if fields["kind"].GetStringValue() != "HTTPRoute" {
			continue
		}
		annotations := fields["annotations"].GetStructValue().GetFields()
		found = append(found, routeInfo{
			Namespace: fields["namespace"].GetStringValue(),
			Name:      fields["name"].GetStringValue(),
			CellName:  annotations["dsh-cell-name"].GetStringValue(),
			CellUID:   annotations["dsh-cell-uid"].GetStringValue(),
		})
	}
	if len(found) != 1 || found[0].Namespace == "" || found[0].Name == "" || found[0].CellName == "" || found[0].CellUID == "" {
		return routeInfo{}, errors.New("exactly one managed HTTPRoute metadata entry is required")
	}
	return found[0], nil
}

func (s *Server) validateRoute(ctx context.Context, info routeInfo, authority string) (*dshv1alpha1.Cell, error) {
	var route gatewayv1.HTTPRoute
	if err := s.Reader.Get(ctx, types.NamespacedName{Namespace: info.Namespace, Name: info.Name}, &route); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errForbidden
		}
		return nil, errUnavailable
	}
	var cell dshv1alpha1.Cell
	if err := s.Reader.Get(ctx, types.NamespacedName{Namespace: info.Namespace, Name: info.CellName}, &cell); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errForbidden
		}
		return nil, errUnavailable
	}
	if cell.DeletionTimestamp != nil || string(cell.UID) != info.CellUID {
		return nil, errForbidden
	}
	names := cellcontract.ResourceNames(string(cell.UID))
	if info.Name != names.Base ||
		route.Annotations[cellcontract.RouteCellNameAnnotation] != cell.Name ||
		route.Annotations[cellcontract.RouteCellUIDAnnotation] != string(cell.UID) ||
		!metav1.IsControlledBy(&route, &cell) ||
		!strings.EqualFold(authority, s.RouteConfig.Authority(string(cell.UID))) ||
		!validRouteSpec(&route, names.Base, s.RouteConfig) {
		return nil, errForbidden
	}
	return &cell, nil
}

func validRouteSpec(route *gatewayv1.HTTPRoute, serviceName string, config accesscontract.Config) bool {
	if len(route.Spec.Hostnames) != 1 || string(route.Spec.Hostnames[0]) != config.Hostname(route.Annotations[cellcontract.RouteCellUIDAnnotation]) {
		return false
	}
	if len(route.Spec.ParentRefs) != 1 {
		return false
	}
	parent := route.Spec.ParentRefs[0]
	if value(parent.Group, gatewayv1.GroupVersion.Group) != gatewayv1.GroupVersion.Group ||
		value(parent.Kind, "Gateway") != "Gateway" ||
		parent.Namespace == nil || string(*parent.Namespace) != config.GatewayNamespace ||
		string(parent.Name) != config.GatewayName ||
		parent.SectionName == nil || string(*parent.SectionName) != config.GatewaySection {
		return false
	}
	if len(route.Spec.Rules) != 1 || len(route.Spec.Rules[0].Matches) != 1 || len(route.Spec.Rules[0].BackendRefs) != 1 || len(route.Spec.Rules[0].Filters) != 0 {
		return false
	}
	match := route.Spec.Rules[0].Matches[0]
	if match.Path == nil || match.Path.Type == nil || *match.Path.Type != gatewayv1.PathMatchPathPrefix || match.Path.Value == nil || *match.Path.Value != "/" || match.Method != nil || len(match.Headers) != 0 || len(match.QueryParams) != 0 {
		return false
	}
	backend := route.Spec.Rules[0].BackendRefs[0]
	if len(backend.Filters) != 0 || value(backend.Group, "") != "" || value(backend.Kind, "Service") != "Service" || string(backend.Name) != serviceName || backend.Namespace != nil || backend.Port == nil || int32(*backend.Port) != cellcontract.ProxyServicePort || (backend.Weight != nil && *backend.Weight != 1) {
		return false
	}
	return true
}

func value[T ~string](pointer *T, fallback string) string {
	if pointer == nil {
		return fallback
	}
	return string(*pointer)
}

type OIDCVerifier struct {
	verifier    *oidc.IDTokenVerifier
	groupsClaim string
}

func NewOIDCVerifier(ctx context.Context, issuer, clientID, groupsClaim string) (*OIDCVerifier, error) {
	providerContext := oidc.ClientContext(ctx, &http.Client{Timeout: 10 * time.Second})
	provider, err := oidc.NewProvider(providerContext, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	return &OIDCVerifier{
		verifier:    provider.Verifier(&oidc.Config{ClientID: clientID}),
		groupsClaim: groupsClaim,
	}, nil
}

func (v *OIDCVerifier) Verify(ctx context.Context, raw string) (Identity, error) {
	token, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		if strings.Contains(err.Error(), "fetching keys") || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return Identity{}, errUnavailable
		}
		return Identity{}, errUnauthenticated
	}
	claims := map[string]any{}
	if err := token.Claims(&claims); err != nil {
		return Identity{}, errUnauthenticated
	}
	subject, ok := claims["sub"].(string)
	if !ok || strings.TrimSpace(subject) == "" {
		return Identity{}, errUnauthenticated
	}
	identity := Identity{Subject: subject}
	if rawGroups, exists := claims[v.groupsClaim]; exists {
		values, ok := rawGroups.([]any)
		if !ok {
			return Identity{}, errUnauthenticated
		}
		identity.Groups = make([]string, 0, len(values))
		for _, rawGroup := range values {
			group, ok := rawGroup.(string)
			if !ok || strings.TrimSpace(group) == "" {
				return Identity{}, errUnauthenticated
			}
			identity.Groups = append(identity.Groups, group)
		}
	}
	return identity, nil
}
