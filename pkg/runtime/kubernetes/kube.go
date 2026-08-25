// Package kubernetes implements the first real runtime backend (①) directly
// against the Kubernetes REST API. Runtime CRs are the desired-state record;
// tenant-owned Pods are the concrete isolation boundary.
package kubernetes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

const (
	managedByLabel = "app.kubernetes.io/managed-by"
	managedByValue = "dsh-isolated-runtime"
	runtimeLabel   = "runtime.dsh.io/name"
	tenantAnno     = "runtime.dsh.io/tenant"
	specHashAnno   = "runtime.dsh.io/spec-hash"
)

type objectMeta struct {
	Name            string            `json:"name,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	ResourceVersion string            `json:"resourceVersion,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
}

type runtimeCRSpec struct {
	Tenant           string                `json:"tenant"`
	RuntimeClass     string                `json:"runtimeClass,omitempty"`
	SecurityClass    runtime.SecurityClass `json:"securityClass,omitempty"`
	Image            string                `json:"image"`
	NetworkIsolation bool                  `json:"networkIsolation,omitempty"`
	ResourceLimits   map[string]string     `json:"resourceLimits,omitempty"`
}

type runtimeCRStatus struct {
	Phase      string `json:"phase,omitempty"`
	RuntimeRef string `json:"runtimeRef,omitempty"`
	Message    string `json:"message,omitempty"`
}

type runtimeCR struct {
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	Metadata   objectMeta      `json:"metadata"`
	Spec       runtimeCRSpec   `json:"spec"`
	Status     runtimeCRStatus `json:"status,omitempty"`
}

type runtimeCRList struct {
	Items []runtimeCR `json:"items"`
}

type pod struct {
	APIVersion string     `json:"apiVersion,omitempty"`
	Kind       string     `json:"kind,omitempty"`
	Metadata   objectMeta `json:"metadata"`
	Spec       podSpec    `json:"spec,omitempty"`
	Status     podStatus  `json:"status,omitempty"`
}

type podList struct {
	Items []pod `json:"items"`
}

type podSpec struct {
	AutomountServiceAccountToken *bool       `json:"automountServiceAccountToken,omitempty"`
	RuntimeClassName              string      `json:"runtimeClassName,omitempty"`
	RestartPolicy                 string      `json:"restartPolicy,omitempty"`
	Containers                    []container `json:"containers"`
}

type container struct {
	Name            string             `json:"name"`
	Image           string             `json:"image"`
	ImagePullPolicy string             `json:"imagePullPolicy,omitempty"`
	SecurityContext containerSecurity  `json:"securityContext"`
	Resources       containerResources `json:"resources,omitempty"`
}

type containerSecurity struct {
	AllowPrivilegeEscalation bool           `json:"allowPrivilegeEscalation"`
	RunAsNonRoot             bool           `json:"runAsNonRoot"`
	Capabilities             capabilities   `json:"capabilities"`
	SeccompProfile           seccompProfile `json:"seccompProfile"`
}

type capabilities struct {
	Drop []string `json:"drop"`
}

type seccompProfile struct {
	Type string `json:"type"`
}

type containerResources struct {
	Limits map[string]string `json:"limits,omitempty"`
}

type podStatus struct {
	Phase             string            `json:"phase,omitempty"`
	PodIP             string            `json:"podIP,omitempty"`
	ContainerStatuses []containerStatus `json:"containerStatuses,omitempty"`
}

type containerStatus struct {
	State struct {
		Waiting *struct {
			Reason  string `json:"reason,omitempty"`
			Message string `json:"message,omitempty"`
		} `json:"waiting,omitempty"`
		Terminated *struct {
			Reason  string `json:"reason,omitempty"`
			Message string `json:"message,omitempty"`
		} `json:"terminated,omitempty"`
	} `json:"state,omitempty"`
}

// Backend persists Runtime desired state in the Runtime CRD and realizes each
// object as one tenant-owned Pod.
type Backend struct {
	client *restClient
}

var _ runtime.Runtime = (*Backend)(nil)

// NewInCluster creates the production Kubernetes backend using the Pod's
// mounted ServiceAccount credentials.
func NewInCluster(namespace string) (*Backend, error) {
	if namespace == "" {
		return nil, fmt.Errorf("kubernetes: runtime namespace is required")
	}
	client, err := newInClusterRESTClient(namespace)
	if err != nil {
		return nil, err
	}
	return &Backend{client: client}, nil
}

func newWithRESTClient(client *restClient) *Backend { return &Backend{client: client} }

func normalizeSpec(spec runtime.Spec) (runtime.Spec, error) {
	if spec.Name == "" || spec.Tenant == "" || spec.Image == "" {
		return runtime.Spec{}, runtime.ErrInvalidSpec
	}
	if spec.SecurityClass == "" {
		spec.SecurityClass = runtime.SecurityStandard
	}
	if spec.SecurityClass != runtime.SecurityStandard && spec.SecurityClass != runtime.SecuritySandboxed {
		return runtime.Spec{}, runtime.ErrInvalidSpec
	}
	if spec.SecurityClass == runtime.SecuritySandboxed && spec.RuntimeClass == "" {
		return runtime.Spec{}, fmt.Errorf("%w: sandboxed runtime requires runtimeClass", runtime.ErrInvalidSpec)
	}
	return spec, nil
}

func (b *Backend) Create(ctx context.Context, spec runtime.Spec) (*runtime.Info, error) {
	spec, err := normalizeSpec(spec)
	if err != nil {
		return nil, err
	}
	obj := runtimeCR{
		APIVersion: "runtime.dsh.io/v1alpha1",
		Kind:       "Runtime",
		Metadata:   objectMeta{Name: spec.Name, Namespace: b.client.namespace},
		Spec: runtimeCRSpec{
			Tenant: spec.Tenant, RuntimeClass: spec.RuntimeClass, SecurityClass: spec.SecurityClass,
			Image: spec.Image, NetworkIsolation: spec.NetworkIsolation, ResourceLimits: spec.ResourceLimits,
		},
	}
	var created runtimeCR
	if err := b.client.do(ctx, http.MethodPost, b.client.runtimeCollectionPath(), obj, &created); err != nil {
		if isStatus(err, http.StatusConflict) {
			return nil, runtime.ErrConflict
		}
		return nil, err
	}
	info := infoFromCR(created, nil)
	return &info, nil
}

func (b *Backend) Get(ctx context.Context, tenant, name string) (*runtime.Info, error) {
	obj, err := b.getRuntime(ctx, name)
	if err != nil {
		if isStatus(err, http.StatusNotFound) {
			return nil, runtime.ErrNotFound
		}
		return nil, err
	}
	if tenant == "" || obj.Spec.Tenant != tenant {
		return nil, runtime.ErrNotFound
	}
	p, err := b.getPod(ctx, name)
	if err != nil && !isStatus(err, http.StatusNotFound) {
		return nil, err
	}
	if isStatus(err, http.StatusNotFound) {
		p = nil
	}
	info := infoFromCR(*obj, p)
	return &info, nil
}

func (b *Backend) List(ctx context.Context, tenant string) ([]runtime.Info, error) {
	var list runtimeCRList
	if err := b.client.do(ctx, http.MethodGet, b.client.runtimeCollectionPath(), nil, &list); err != nil {
		return nil, err
	}
	out := make([]runtime.Info, 0, len(list.Items))
	for _, obj := range list.Items {
		if tenant != "" && obj.Spec.Tenant != tenant {
			continue
		}
		p, err := b.getPod(ctx, obj.Metadata.Name)
		if err != nil && !isStatus(err, http.StatusNotFound) {
			return nil, err
		}
		if isStatus(err, http.StatusNotFound) {
			p = nil
		}
		out = append(out, infoFromCR(obj, p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (b *Backend) Delete(ctx context.Context, tenant, name string) error {
	obj, err := b.getRuntime(ctx, name)
	if err != nil {
		if isStatus(err, http.StatusNotFound) {
			return runtime.ErrNotFound
		}
		return err
	}
	if obj.Spec.Tenant != tenant {
		return runtime.ErrNotFound
	}
	if err := b.client.do(ctx, http.MethodDelete, b.client.runtimePath(name), nil, nil); err != nil && !isStatus(err, http.StatusNotFound) {
		return err
	}
	_ = b.deletePod(ctx, name)
	_ = b.deleteNetworkPolicy(ctx, name)
	return nil
}

// ReconcileAll realizes every Runtime CR as one Pod and removes managed orphan
// Pods whose desired-state Runtime no longer exists.
func (b *Backend) ReconcileAll(ctx context.Context) error {
	var list runtimeCRList
	if err := b.client.do(ctx, http.MethodGet, b.client.runtimeCollectionPath(), nil, &list); err != nil {
		return err
	}
	desired := make(map[string]struct{}, len(list.Items))
	var errs []error
	for i := range list.Items {
		desired[list.Items[i].Metadata.Name] = struct{}{}
		if err := b.reconcileRuntime(ctx, &list.Items[i]); err != nil {
			errs = append(errs, fmt.Errorf("reconcile %s: %w", list.Items[i].Metadata.Name, err))
		}
	}
	pods, err := b.listManagedPods(ctx)
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, p := range pods {
			if _, ok := desired[p.Metadata.Name]; !ok {
				if err := b.deletePod(ctx, p.Metadata.Name); err != nil {
					errs = append(errs, err)
				}
				_ = b.deleteNetworkPolicy(ctx, p.Metadata.Name)
			}
		}
	}
	return errors.Join(errs...)
}

func (b *Backend) reconcileRuntime(ctx context.Context, obj *runtimeCR) error {
	spec := runtime.Spec{
		Name: obj.Metadata.Name, Tenant: obj.Spec.Tenant, RuntimeClass: obj.Spec.RuntimeClass,
		SecurityClass: obj.Spec.SecurityClass, Image: obj.Spec.Image,
		NetworkIsolation: obj.Spec.NetworkIsolation, ResourceLimits: obj.Spec.ResourceLimits,
	}
	normalized, err := normalizeSpec(spec)
	if err != nil {
		_ = b.updateStatus(ctx, obj, "Failed", "", err.Error())
		return nil
	}
	p, err := b.getPod(ctx, obj.Metadata.Name)
	if err != nil && !isStatus(err, http.StatusNotFound) {
		return err
	}
	desiredHash := hashSpec(normalized)
	if p != nil && p.Metadata.Annotations[specHashAnno] != desiredHash {
		if err := b.deletePod(ctx, obj.Metadata.Name); err != nil {
			return err
		}
		p = nil
	}
	if p == nil {
		if normalized.NetworkIsolation {
			if err := b.ensureNetworkPolicy(ctx, normalized.Name); err != nil {
				return err
			}
		} else {
			_ = b.deleteNetworkPolicy(ctx, normalized.Name)
		}
		candidate := podForSpec(b.client.namespace, normalized, desiredHash)
		if err := b.client.do(ctx, http.MethodPost, b.client.podCollectionPath(), candidate, &candidate); err != nil {
			if !isStatus(err, http.StatusConflict) {
				return err
			}
		}
		_ = b.updateStatus(ctx, obj, "Pending", "pod://"+normalized.Name, "Pod created")
		return nil
	}
	info := infoFromCR(*obj, p)
	return b.updateStatus(ctx, obj, info.Phase, info.Address, info.Message)
}

func (b *Backend) getRuntime(ctx context.Context, name string) (*runtimeCR, error) {
	var obj runtimeCR
	if err := b.client.do(ctx, http.MethodGet, b.client.runtimePath(name), nil, &obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

func (b *Backend) getPod(ctx context.Context, name string) (*pod, error) {
	var p pod
	if err := b.client.do(ctx, http.MethodGet, b.client.podPath(name), nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (b *Backend) listManagedPods(ctx context.Context) ([]pod, error) {
	path := b.client.podCollectionPath() + "?labelSelector=" + url.QueryEscape(managedByLabel+"="+managedByValue)
	var list podList
	if err := b.client.do(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (b *Backend) deletePod(ctx context.Context, name string) error {
	err := b.client.do(ctx, http.MethodDelete, b.client.podPath(name), nil, nil)
	if isStatus(err, http.StatusNotFound) {
		return nil
	}
	return err
}

func (b *Backend) ensureNetworkPolicy(ctx context.Context, name string) error {
	var existing map[string]any
	if err := b.client.do(ctx, http.MethodGet, b.client.networkPolicyPath(name), nil, &existing); err == nil {
		return nil
	} else if !isStatus(err, http.StatusNotFound) {
		return err
	}
	policy := map[string]any{
		"apiVersion": "networking.k8s.io/v1", "kind": "NetworkPolicy",
		"metadata": map[string]any{"name": name, "namespace": b.client.namespace, "labels": map[string]string{managedByLabel: managedByValue, runtimeLabel: name}},
		"spec": map[string]any{"podSelector": map[string]any{"matchLabels": map[string]string{runtimeLabel: name}}, "policyTypes": []string{"Ingress", "Egress"}},
	}
	return b.client.do(ctx, http.MethodPost, b.client.networkPolicyCollectionPath(), policy, nil)
}

func (b *Backend) deleteNetworkPolicy(ctx context.Context, name string) error {
	err := b.client.do(ctx, http.MethodDelete, b.client.networkPolicyPath(name), nil, nil)
	if isStatus(err, http.StatusNotFound) {
		return nil
	}
	return err
}

func (b *Backend) updateStatus(ctx context.Context, obj *runtimeCR, phase, ref, message string) error {
	if obj.Status.Phase == phase && obj.Status.RuntimeRef == ref && obj.Status.Message == message {
		return nil
	}
	copy := *obj
	copy.Status = runtimeCRStatus{Phase: phase, RuntimeRef: ref, Message: message}
	var updated runtimeCR
	if err := b.client.do(ctx, http.MethodPut, b.client.runtimePath(obj.Metadata.Name)+"/status", copy, &updated); err != nil {
		if isStatus(err, http.StatusConflict) {
			return nil
		}
		return err
	}
	*obj = updated
	return nil
}

func podForSpec(namespace string, spec runtime.Spec, hash string) pod {
	automount := false
	labels := map[string]string{managedByLabel: managedByValue, runtimeLabel: spec.Name}
	annotations := map[string]string{tenantAnno: spec.Tenant, specHashAnno: hash}
	return pod{
		APIVersion: "v1", Kind: "Pod",
		Metadata: objectMeta{Name: spec.Name, Namespace: namespace, Labels: labels, Annotations: annotations},
		Spec: podSpec{
			AutomountServiceAccountToken: &automount,
			RuntimeClassName:              spec.RuntimeClass,
			RestartPolicy:                 "Always",
			Containers: []container{{
				Name: "runtime", Image: spec.Image, ImagePullPolicy: "IfNotPresent",
				SecurityContext: containerSecurity{
					AllowPrivilegeEscalation: false, RunAsNonRoot: true,
					Capabilities: capabilities{Drop: []string{"ALL"}},
					SeccompProfile: seccompProfile{Type: "RuntimeDefault"},
				},
				Resources: containerResources{Limits: spec.ResourceLimits},
			}},
		},
	}
}

func hashSpec(spec runtime.Spec) string {
	data, _ := json.Marshal(spec)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}

func infoFromCR(obj runtimeCR, p *pod) runtime.Info {
	info := runtime.Info{
		Name: obj.Metadata.Name, Tenant: obj.Spec.Tenant, RuntimeClass: obj.Spec.RuntimeClass,
		SecurityClass: obj.Spec.SecurityClass, Image: obj.Spec.Image, Phase: "Pending",
		Address: "pod://" + obj.Metadata.Name,
	}
	if info.SecurityClass == "" {
		info.SecurityClass = runtime.SecurityStandard
	}
	if p == nil {
		if obj.Status.Phase != "" {
			info.Phase = obj.Status.Phase
			info.Message = obj.Status.Message
		}
		return info
	}
	switch p.Status.Phase {
	case "Running":
		info.Phase = "Running"
	case "Failed", "Succeeded":
		info.Phase = "Failed"
	case "Pending", "":
		info.Phase = "Pending"
	default:
		info.Phase = p.Status.Phase
	}
	if p.Status.PodIP != "" {
		info.Address = "pod://" + obj.Metadata.Name + "@" + p.Status.PodIP
	}
	for _, status := range p.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			info.Message = strings.TrimSpace(status.State.Waiting.Reason + ": " + status.State.Waiting.Message)
			break
		}
		if status.State.Terminated != nil {
			info.Message = strings.TrimSpace(status.State.Terminated.Reason + ": " + status.State.Terminated.Message)
			break
		}
	}
	return info
}
