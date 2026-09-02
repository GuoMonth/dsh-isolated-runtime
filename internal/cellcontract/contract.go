// Package cellcontract is the single source of truth shared by the Cell
// controller, launcher image, manifests, and vertical-slice tests.
package cellcontract

import "fmt"

const (
	ContractVersion = "v1alpha1"
	DSHVersion      = "0.1.2-alpha.4"

	ContainerName = "cell"
	LauncherPath  = "/usr/local/bin/cell-launcher"
	NodePath      = "/usr/local/bin/node"
	DSHPath       = "/opt/dsh/node_modules/.bin/dsh"
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

	ManagedByLabel     = "app.kubernetes.io/managed-by"
	ApplicationLabel   = "app.kubernetes.io/name"
	CellUIDLabel       = "dsh.isolated.io/cell-uid"
	AccessLabel        = "dsh.isolated.io/access"
	CellNameAnnotation = "dsh.isolated.io/cell-name"
	CellUIDAnnotation  = "dsh.isolated.io/cell-uid"

	ManagedByValue   = "dsh-isolated-runtime"
	ApplicationValue = "dsh-cell"
	AccessValue      = "true"
)

// Names are all native Kubernetes names derived from one immutable Cell UID.
type Names struct {
	Base       string
	DataPVC    string
	PrivatePVC string
	Headless   string
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
