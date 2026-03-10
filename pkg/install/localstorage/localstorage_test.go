package localstorage

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
)

func newLocalStorageSpec(tolerationOnMaster bool) *hwameistoriov1alpha1.LocalStorageSpec {
	return &hwameistoriov1alpha1.LocalStorageSpec{
		KubeletRootDir:     "/var/lib/kubelet",
		TolerationOnMaster: tolerationOnMaster,
		Common: &hwameistoriov1alpha1.PodCommonSpec{
			Tolerations: &[]corev1.Toleration{{
				Key:      "dedicated",
				Operator: corev1.TolerationOpEqual,
				Value:    "storage",
				Effect:   corev1.TaintEffectNoSchedule,
			}},
		},
		Member: &hwameistoriov1alpha1.MemberSpec{
			HostPathDRBDDir: "/etc/drbd.d",
			HostPathSSHDir:  "/root/.ssh",
			Image:           &hwameistoriov1alpha1.ImageSpec{},
			JuicesyncImage:  &hwameistoriov1alpha1.ImageSpec{},
		},
		CSI: &hwameistoriov1alpha1.CSISpec{
			Registrar: &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
			Controller: &hwameistoriov1alpha1.CSIControllerSpec{
				Provisioner:        &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
				Attacher:           &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
				Resizer:            &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
				Monitor:            &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
				SnapshotController: &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
				Snapshotter:        &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
			},
		},
	}
}

func TestSetLSDaemonSetUsesCommonTolerations(t *testing.T) {
	cluster := &hwameistoriov1alpha1.Cluster{}
	cluster.Spec.TargetNamespace = "hwameistor"
	cluster.Spec.RBAC = &hwameistoriov1alpha1.RBACSpec{ServiceAccountName: "hwameistor-admin"}
	cluster.Spec.LocalStorage = newLocalStorageSpec(false)

	daemonSet := SetLSDaemonSet(cluster)
	if len(daemonSet.Spec.Template.Spec.Tolerations) != 1 {
		t.Fatalf("expected 1 toleration, got %d", len(daemonSet.Spec.Template.Spec.Tolerations))
	}
	if daemonSet.Spec.Template.Spec.Tolerations[0].Key != "dedicated" {
		t.Fatalf("expected custom toleration to be propagated, got %#v", daemonSet.Spec.Template.Spec.Tolerations)
	}
}

func TestSetLSDaemonSetAppendsMasterTolerations(t *testing.T) {
	cluster := &hwameistoriov1alpha1.Cluster{}
	cluster.Spec.TargetNamespace = "hwameistor"
	cluster.Spec.RBAC = &hwameistoriov1alpha1.RBACSpec{ServiceAccountName: "hwameistor-admin"}
	cluster.Spec.LocalStorage = newLocalStorageSpec(true)

	daemonSet := SetLSDaemonSet(cluster)
	if len(daemonSet.Spec.Template.Spec.Tolerations) <= 1 {
		t.Fatalf("expected custom + master tolerations, got %#v", daemonSet.Spec.Template.Spec.Tolerations)
	}
	if daemonSet.Spec.Template.Spec.Tolerations[0].Key != "dedicated" {
		t.Fatalf("expected custom toleration first, got %#v", daemonSet.Spec.Template.Spec.Tolerations)
	}
}
