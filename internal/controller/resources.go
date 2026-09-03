package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

type ownershipConflictError struct {
	resource string
}

func (e *ownershipConflictError) Error() string {
	return fmt.Sprintf("foreign %s occupies a managed Cell resource name", e.resource)
}

func (r *CellReconciler) reconcileDataPVC(
	ctx context.Context,
	cell *dshv1alpha1.Cell,
) (*corev1.PersistentVolumeClaim, error) {
	names := cellcontract.ResourceNames(string(cell.UID))
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: names.DataPVC, Namespace: cell.Namespace}}
	controlled := cell.Spec.Storage.RetentionPolicy == dshv1alpha1.RetentionDelete
	err := r.ensureManaged(ctx, cell, claim, controlled, func(created bool) error {
		if created {
			claim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			claim.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: cell.Spec.Storage.Size.DeepCopy()}
			claim.Spec.StorageClassName = copyString(cell.Spec.Storage.StorageClassName)
			return nil
		}
		if !singleRWO(claim.Spec.AccessModes) {
			return errors.New("managed data PVC access mode drifted")
		}
		if cell.Spec.Storage.StorageClassName != nil && !equalString(claim.Spec.StorageClassName, cell.Spec.Storage.StorageClassName) {
			return errors.New("managed data PVC StorageClass drifted")
		}
		current := claim.Spec.Resources.Requests[corev1.ResourceStorage]
		if current.Cmp(cell.Spec.Storage.Size) < 0 {
			if claim.Spec.Resources.Requests == nil {
				claim.Spec.Resources.Requests = corev1.ResourceList{}
			}
			claim.Spec.Resources.Requests[corev1.ResourceStorage] = cell.Spec.Storage.Size.DeepCopy()
		}
		return nil
	})
	return claim, err
}

func (r *CellReconciler) reconcilePrivatePVC(
	ctx context.Context,
	cell *dshv1alpha1.Cell,
	storageClassName *string,
) (*corev1.PersistentVolumeClaim, error) {
	names := cellcontract.ResourceNames(string(cell.UID))
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: names.PrivatePVC, Namespace: cell.Namespace}}
	minimum := resource.MustParse(cellcontract.PrivatePVCSize)
	err := r.ensureManaged(ctx, cell, claim, true, func(created bool) error {
		if created {
			claim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			claim.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: minimum}
			claim.Spec.StorageClassName = copyString(storageClassName)
			return nil
		}
		if !singleRWO(claim.Spec.AccessModes) {
			return errors.New("managed private PVC access mode drifted")
		}
		if storageClassName != nil && !equalString(claim.Spec.StorageClassName, storageClassName) {
			return errors.New("managed private PVC StorageClass drifted")
		}
		current := claim.Spec.Resources.Requests[corev1.ResourceStorage]
		if current.Cmp(minimum) < 0 {
			return errors.New("managed private PVC is smaller than the workload contract")
		}
		return nil
	})
	return claim, err
}

func (r *CellReconciler) reconcileServiceAccount(ctx context.Context, cell *dshv1alpha1.Cell) error {
	names := cellcontract.ResourceNames(string(cell.UID))
	account := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}}
	return r.ensureManaged(ctx, cell, account, true, func(_ bool) error {
		account.AutomountServiceAccountToken = ptr.To(false)
		return nil
	})
}

func (r *CellReconciler) reconcileHeadlessService(ctx context.Context, cell *dshv1alpha1.Cell) error {
	names := cellcontract.ResourceNames(string(cell.UID))
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: names.Headless, Namespace: cell.Namespace}}
	return r.ensureManaged(ctx, cell, service, true, func(created bool) error {
		if created {
			service.Spec.ClusterIP = corev1.ClusterIPNone
		}
		if service.Spec.ClusterIP != corev1.ClusterIPNone {
			return errors.New("managed governing Service is not headless")
		}
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Selector = workloadSelector(cell)
		service.Spec.Ports = []corev1.ServicePort{{
			Name:       cellcontract.ProxyPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       cellcontract.ProxyContainerPort,
			TargetPort: intstr.FromString(cellcontract.ProxyPortName),
		}}
		return nil
	})
}

func (r *CellReconciler) reconcileAccessService(ctx context.Context, cell *dshv1alpha1.Cell) error {
	names := cellcontract.ResourceNames(string(cell.UID))
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}}
	return r.ensureManaged(ctx, cell, service, true, func(created bool) error {
		if !created && service.Spec.ClusterIP == corev1.ClusterIPNone {
			return errors.New("managed access Service is unexpectedly headless")
		}
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Selector = workloadSelector(cell)
		service.Spec.Ports = []corev1.ServicePort{{
			Name:       cellcontract.ProxyPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       cellcontract.ProxyServicePort,
			TargetPort: intstr.FromString(cellcontract.ProxyPortName),
		}}
		return nil
	})
}

func (r *CellReconciler) reconcileStatefulSet(
	ctx context.Context,
	cell *dshv1alpha1.Cell,
) (*appsv1.StatefulSet, error) {
	names := cellcontract.ResourceNames(string(cell.UID))
	workload := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}}
	err := r.ensureManaged(ctx, cell, workload, true, func(created bool) error {
		selector := workloadSelector(cell)
		if !created && !reflect.DeepEqual(workload.Spec.Selector.MatchLabels, selector) {
			return errors.New("managed StatefulSet selector drifted")
		}
		workload.Spec.Replicas = ptr.To[int32](1)
		workload.Spec.ServiceName = names.Headless
		workload.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
		workload.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType}
		workload.Spec.RevisionHistoryLimit = ptr.To[int32](2)
		workload.Spec.Selector = &metav1.LabelSelector{MatchLabels: selector}
		workload.Spec.Template = r.desiredPodTemplate(cell)
		return nil
	})
	return workload, err
}

func (r *CellReconciler) deleteStatefulSet(ctx context.Context, cell *dshv1alpha1.Cell) error {
	names := cellcontract.ResourceNames(string(cell.UID))
	workload := &appsv1.StatefulSet{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: cell.Namespace, Name: names.Base}, workload); err != nil {
		return client.IgnoreNotFound(err)
	}
	if err := validateManaged(cell, workload, true); err != nil {
		return err
	}
	if workload.DeletionTimestamp != nil {
		return nil
	}
	return r.Delete(ctx, workload)
}

func (r *CellReconciler) reconcileNetworkPolicy(ctx context.Context, cell *dshv1alpha1.Cell) error {
	names := cellcontract.ResourceNames(string(cell.UID))
	policy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}}
	return r.ensureManaged(ctx, cell, policy, true, func(_ bool) error {
		protocol := corev1.ProtocolTCP
		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: workloadSelector(cell)},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
						corev1.LabelMetadataName: r.SystemNamespace,
					}},
					PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
						cellcontract.AccessLabel: cellcontract.AccessValue,
					}},
				}},
				Ports: []networkingv1.NetworkPolicyPort{{
					Protocol: &protocol,
					Port:     ptr.To(intstr.FromInt32(cellcontract.ProxyContainerPort)),
				}},
			}},
		}
		return nil
	})
}

func (r *CellReconciler) desiredPodTemplate(cell *dshv1alpha1.Cell) corev1.PodTemplateSpec {
	names := cellcontract.ResourceNames(string(cell.UID))
	nonRoot := true
	allowPrivilegeEscalation := false
	readOnlyRoot := true
	fsGroupChangePolicy := corev1.FSGroupChangeOnRootMismatch
	temporarySize := resource.MustParse(cellcontract.TemporarySize)

	environment := []corev1.EnvVar{
		{Name: "CELL_AUTHORITY", Value: r.cellAuthority(cell)},
		{Name: "HOME", Value: cellcontract.DSHHome},
		{Name: "DSH_HOME", Value: cellcontract.DSHHome},
		{Name: "DSH_AGENTS_HOME", Value: cellcontract.AgentsHome},
		{Name: "DSH_TELEMETRY_DISABLED", Value: "1"},
		{Name: "XDG_CACHE_HOME", Value: cellcontract.TemporaryRoot + "/.cache"},
	}
	var envFrom []corev1.EnvFromSource
	if cell.Spec.CredentialsRef != nil {
		envFrom = []corev1.EnvFromSource{{
			SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: cell.Spec.CredentialsRef.Name}},
		}}
	}

	resources := corev1.ResourceRequirements{}
	if cell.Spec.Resources.Limits != nil {
		resources.Limits = cell.Spec.Resources.Limits.DeepCopy()
	}
	if cell.Spec.Resources.Requests != nil {
		resources.Requests = cell.Spec.Resources.Requests.DeepCopy()
	}

	podSpec := corev1.PodSpec{
		ServiceAccountName:            names.Base,
		AutomountServiceAccountToken:  ptr.To(false),
		EnableServiceLinks:            ptr.To(false),
		TerminationGracePeriodSeconds: ptr.To[int64](30),
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot:        &nonRoot,
			RunAsUser:           ptr.To[int64](1000),
			RunAsGroup:          ptr.To[int64](1000),
			FSGroup:             ptr.To[int64](1000),
			FSGroupChangePolicy: &fsGroupChangePolicy,
			SeccompProfile:      &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
		Containers: []corev1.Container{{
			Name:            cellcontract.ContainerName,
			Image:           cell.Spec.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{cellcontract.LauncherPath},
			// The mounted data root exists before process start. The launcher
			// creates Workspace and uses it as the DSH child's working directory.
			WorkingDir: cellcontract.DataRoot,
			Env:        environment,
			EnvFrom:    envFrom,
			Resources:  resources,
			Ports: []corev1.ContainerPort{
				{Name: cellcontract.ProxyPortName, ContainerPort: cellcontract.ProxyContainerPort, Protocol: corev1.ProtocolTCP},
				{Name: cellcontract.ManagementPortName, ContainerPort: cellcontract.ManagementPort, Protocol: corev1.ProtocolTCP},
			},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: &allowPrivilegeEscalation,
				ReadOnlyRootFilesystem:   &readOnlyRoot,
				RunAsNonRoot:             &nonRoot,
				RunAsUser:                ptr.To[int64](1000),
				RunAsGroup:               ptr.To[int64](1000),
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			},
			StartupProbe:   httpProbe("/readyz", 3, 30),
			ReadinessProbe: httpProbe("/readyz", 3, 3),
			LivenessProbe:  httpProbe("/livez", 10, 3),
			VolumeMounts: []corev1.VolumeMount{
				{Name: cellcontract.DataVolumeName, MountPath: cellcontract.DataRoot},
				{Name: cellcontract.PrivateVolumeName, MountPath: cellcontract.PrivateRoot},
				{Name: cellcontract.TemporaryVolumeName, MountPath: cellcontract.TemporaryRoot},
			},
		}},
		Volumes: []corev1.Volume{
			{
				Name:         cellcontract.DataVolumeName,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: names.DataPVC}},
			},
			{
				Name:         cellcontract.PrivateVolumeName,
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: names.PrivatePVC}},
			},
			{
				Name:         cellcontract.TemporaryVolumeName,
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &temporarySize}},
			},
		},
	}
	if cell.Spec.SecurityClass == dshv1alpha1.SecuritySandboxed {
		// The reconciler has already rejected an empty mapping. RuntimeClass
		// remains cluster-owned and outside the Cell API.
		podSpec.RuntimeClassName = ptr.To(r.SandboxedRuntimeClass)
	}

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: workloadSelector(cell), Annotations: cellAnnotations(cell)},
		Spec:       podSpec,
	}
}

func (r *CellReconciler) ensureManaged(
	ctx context.Context,
	cell *dshv1alpha1.Cell,
	object client.Object,
	controlled bool,
	mutate func(created bool) error,
) error {
	key := client.ObjectKeyFromObject(object)
	err := r.Get(ctx, key, object)
	created := apierrors.IsNotFound(err)
	if err != nil && !created {
		return err
	}
	if !created {
		if err := validateManaged(cell, object, controlled); err != nil {
			return err
		}
	}
	_, err = controllerutil.CreateOrPatch(ctx, r.Client, object, func() error {
		setCellMetadata(object, cell)
		if controlled {
			if err := controllerutil.SetControllerReference(cell, object, r.Scheme); err != nil {
				return err
			}
		}
		return mutate(created)
	})
	return err
}

func validateManaged(cell *dshv1alpha1.Cell, object client.Object, controlled bool) error {
	annotations := object.GetAnnotations()
	if annotations[cellcontract.CellUIDAnnotation] != string(cell.UID) || annotations[cellcontract.CellNameAnnotation] != cell.Name {
		return &ownershipConflictError{resource: fmt.Sprintf("%T", object)}
	}
	controller := metav1.GetControllerOf(object)
	if controlled {
		if controller == nil || controller.UID != cell.UID || controller.Kind != "Cell" || controller.APIVersion != dshv1alpha1.GroupVersion.String() {
			return &ownershipConflictError{resource: fmt.Sprintf("%T", object)}
		}
	} else if len(object.GetOwnerReferences()) != 0 {
		return &ownershipConflictError{resource: fmt.Sprintf("%T", object)}
	}
	return nil
}

func setCellMetadata(object client.Object, cell *dshv1alpha1.Cell) {
	labels := object.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for key, value := range workloadSelector(cell) {
		labels[key] = value
	}
	object.SetLabels(labels)
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	for key, value := range cellAnnotations(cell) {
		annotations[key] = value
	}
	object.SetAnnotations(annotations)
}

func workloadSelector(cell *dshv1alpha1.Cell) map[string]string {
	return map[string]string{
		cellcontract.ApplicationLabel: cellcontract.ApplicationValue,
		cellcontract.ManagedByLabel:   cellcontract.ManagedByValue,
		cellcontract.CellUIDLabel:     string(cell.UID),
	}
}

func cellAnnotations(cell *dshv1alpha1.Cell) map[string]string {
	return map[string]string{
		cellcontract.CellNameAnnotation: cell.Name,
		cellcontract.CellUIDAnnotation:  string(cell.UID),
	}
}

func httpProbe(path string, period, failures int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
			Path: path,
			Port: intstr.FromString(cellcontract.ManagementPortName),
		}},
		TimeoutSeconds:   1,
		PeriodSeconds:    period,
		FailureThreshold: failures,
		SuccessThreshold: 1,
	}
}

func singleRWO(modes []corev1.PersistentVolumeAccessMode) bool {
	return len(modes) == 1 && modes[0] == corev1.ReadWriteOnce
}

func equalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func copyString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
