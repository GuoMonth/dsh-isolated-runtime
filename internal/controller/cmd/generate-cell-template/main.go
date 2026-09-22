// Command generate-cell-template writes the Connector's fixed template from
// the same renderer used by the Cell controller.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/controller"
)

const (
	fixtureUID       = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	fixtureName      = "cell-template"
	fixtureAuthority = "__ORIGIN_HOST__"
	fixtureImage     = "__IMAGE__"
	fixtureSecret    = "__CREDENTIALS_SECRET__"
)

func main() {
	cell := &dshv1alpha1.Cell{
		ObjectMeta: metav1.ObjectMeta{Name: fixtureName, Namespace: "template-ns", UID: fixtureUID},
		Spec: dshv1alpha1.CellSpec{
			Image:         fixtureImage,
			SecurityClass: dshv1alpha1.SecurityStandard,
			Resources: dshv1alpha1.CellResources{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			Storage: dshv1alpha1.CellStorageSpec{
				Size:             resource.MustParse("20Gi"),
				StorageClassName: ptr.To("__STORAGE_CLASS__"),
				RetentionPolicy:  dshv1alpha1.RetentionRetain,
			},
			CredentialsRef: &dshv1alpha1.LocalSecretReference{Name: fixtureSecret},
		},
	}

	cellSpec := toObject(cell.Spec)
	set(cellSpec, "image", fixtureImage)
	set(cellSpec, "resources", map[string]any{
		"requests": map[string]any{"cpu": "__CPU_REQUEST__", "memory": "__MEMORY_REQUEST__"},
		"limits":   map[string]any{"cpu": "__CPU_LIMIT__", "memory": "__MEMORY_LIMIT__"},
	})
	set(cellSpec, "storage.size", "__STORAGE_SIZE__")
	set(cellSpec, "storage.storageClassName", "__STORAGE_CLASS__")
	set(cellSpec, "storage.retentionPolicy", "__RETENTION_POLICY__")
	set(cellSpec, "credentialsRef.name", fixtureSecret)

	podTemplate := toObject(controller.DesiredPodTemplate(cell, fixtureAuthority))
	set(podTemplate, "spec.containers.0.image", fixtureImage)
	set(podTemplate, "spec.containers.0.resources", map[string]any{
		"requests": map[string]any{"cpu": "__CPU_REQUEST__", "memory": "__MEMORY_REQUEST__"},
		"limits":   map[string]any{"cpu": "__CPU_LIMIT__", "memory": "__MEMORY_LIMIT__"},
	})
	set(podTemplate, "spec.volumes.0.persistentVolumeClaim.claimName", "cell-__CELL_UID__-data")
	set(podTemplate, "spec.volumes.1.persistentVolumeClaim.claimName", "cell-__CELL_UID__-private")
	set(podTemplate, "spec.serviceAccountName", "cell-__CELL_UID__")
	set(podTemplate, "spec.containers.0.env.0.value", fixtureAuthority)
	set(podTemplate, "spec.containers.0.envFrom.0.secretRef.name", fixtureSecret)
	// UID-derived names and metadata values become explicit runtime substitutions.
	podTemplate = replaceStrings(podTemplate, strings.NewReplacer(fixtureUID, "__CELL_UID__", fixtureName, "__CELL_NAME__")).(map[string]any)

	result := map[string]any{
		"version":     "cell-mvp-v1",
		"cellSpec":    cellSpec,
		"podTemplate": podTemplate,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func toObject(value any) map[string]any {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}

func set(value map[string]any, path string, replacement any) {
	parts := strings.Split(path, ".")
	var current any = value
	for _, part := range parts[:len(parts)-1] {
		switch typed := current.(type) {
		case map[string]any:
			current = typed[part]
		case []any:
			var index int
			if _, err := fmt.Sscanf(part, "%d", &index); err != nil || index < 0 || index >= len(typed) {
				panic("invalid template path: " + path)
			}
			current = typed[index]
		default:
			panic("invalid template path: " + path)
		}
	}
	last := parts[len(parts)-1]
	if object, ok := current.(map[string]any); ok {
		object[last] = replacement
		return
	}
	panic("invalid template path: " + path)
}

func replaceStrings(value any, replacer *strings.Replacer) any {
	switch typed := value.(type) {
	case string:
		return replacer.Replace(typed)
	case map[string]any:
		for key, child := range typed {
			typed[key] = replaceStrings(child, replacer)
		}
	case []any:
		for index, child := range typed {
			typed[index] = replaceStrings(child, replacer)
		}
	}
	return value
}
