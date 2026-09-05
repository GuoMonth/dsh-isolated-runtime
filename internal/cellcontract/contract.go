// Package cellcontract is the single source of truth shared by the Cell
// controller, launcher image, manifests, and vertical-slice tests.
package cellcontract

import (
	"fmt"
	"strings"
)

const (
	ContractVersion = "v1alpha1"
	DSHVersion      = "0.1.3-alpha.1"

	ContainerName = "cell"
	LauncherPath  = "/usr/local/bin/cell-launcher"
	NodePath      = "/usr/local/bin/node"
	DSHPath       = "/opt/dsh/node_modules/@deepseek-ai/dsh/lib/bin.js"
	PatchPath     = "/etc/dsh/cell.patch.yml"

	ProxyPortName      = "http"
	ProxyContainerPort = 8080
	ProxyServicePort   = 80
	ManagementPortName = "management"
	ManagementPort     = 8081

	DataRoot        = "/var/lib/dsh/data"
	DSHHome         = DataRoot + "/home"
	AgentsHome      = DataRoot + "/agents"
	Workspace       = DataRoot + "/workspace"
	PrivateRoot     = "/var/lib/dsh-private"
	CredentialsPath = PrivateRoot + "/.credentials.yaml"
	TemporaryRoot   = "/tmp"
	PrivatePVCSize  = "1Gi"
	TemporarySize   = "1Gi"

	DataVolumeName      = "data"
	PrivateVolumeName   = "private"
	TemporaryVolumeName = "tmp"

	ManagedByLabel                   = "app.kubernetes.io/managed-by"
	ApplicationLabel                 = "app.kubernetes.io/name"
	CellUIDLabel                     = "dsh.isolated.io/cell-uid"
	AccessLabel                      = "dsh.isolated.io/access"
	CellNameAnnotation               = "dsh.isolated.io/cell-name"
	CellUIDAnnotation                = "dsh.isolated.io/cell-uid"
	SnapshotUIDAnnotation            = "dsh.isolated.io/cell-snapshot-uid"
	RouteCellNameAnnotation          = "gateway.envoyproxy.io/dsh-cell-name"
	RouteCellUIDAnnotation           = "gateway.envoyproxy.io/dsh-cell-uid"
	ActiveSnapshotAnnotation         = "dsh.isolated.io/active-snapshot-uid"
	RestoreSnapshotUIDAnnotation     = "dsh.isolated.io/restore-snapshot-uid"
	RestoreImageDigestAnnotation     = "dsh.isolated.io/restore-image-digest"
	RestoreDSHVersionAnnotation      = "dsh.isolated.io/restore-dsh-version"
	RestoreInitializedAnnotation     = "dsh.isolated.io/restore-initialized"
	OIDCTokenHeader                  = "X-Dsh-Oidc-Token"
	SnapshotFinalizer                = "dsh.isolated.io/snapshot-protection"
	RestoreInitializationFinalizer   = "dsh.isolated.io/restore-initialization"
	RestoreProtectionFinalizerPrefix = "dsh.isolated.io/restore-"

	ManagedByValue   = "dsh-isolated-runtime"
	ApplicationValue = "dsh-cell"
	OperatorValue    = "cell-operator"
	AccessValue      = "true"
)

// RestoreProtectionFinalizer keeps one Ready CellSnapshot and its CSI object
// alive until the target Cell's exact recorded image becomes the first Ready
// reader. The API-server-issued Cell UID makes concurrent restores distinct.
func RestoreProtectionFinalizer(cellUID string) string {
	return RestoreProtectionFinalizerPrefix + cellUID
}

// Names are all native Kubernetes names derived from one immutable Cell UID.
type Names struct {
	Base       string
	DataPVC    string
	PrivatePVC string
	Headless   string
}

// SnapshotName is the namespaced CSI VolumeSnapshot name derived from one
// API-server-issued CellSnapshot UID.
func SnapshotName(uid string) string {
	return "cellsnapshot-" + uid
}

// ResourceNames returns DNS-label-safe names for an API-server-issued UUID UID.
func ResourceNames(uid string) Names {
	base := "cell-" + uid
	return Names{
		Base:       base,
		DataPVC:    base + "-data",
		PrivatePVC: base + "-private",
		Headless:   base + "-headless",
	}
}

// Authority is the portless Phase 1 internal Service authority.
func Authority(namespace, uid string) string {
	return fmt.Sprintf("%s.%s.svc", ResourceNames(uid).Base, namespace)
}

// PublicHostname is the topology-free DNS identity derived from a Cell UID.
func PublicHostname(baseDomain, uid string) string {
	return fmt.Sprintf("%s.%s", ResourceNames(uid).Base, strings.TrimSuffix(baseDomain, "."))
}

// PublicAuthority is the browser authority. HTTPS uses the implicit port 443.
func PublicAuthority(baseDomain, uid string, port int) string {
	hostname := PublicHostname(baseDomain, uid)
	if port == 443 {
		return hostname
	}
	return fmt.Sprintf("%s:%d", hostname, port)
}
