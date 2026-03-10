package localdiskmanager

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
)

func TestSetLDMDaemonSetUsesCommonTolerations(t *testing.T) {
	cluster := &hwameistoriov1alpha1.Cluster{}
	cluster.Spec.TargetNamespace = "hwameistor"
	cluster.Spec.RBAC = &hwameistoriov1alpha1.RBACSpec{ServiceAccountName: "hwameistor-admin"}
	cluster.Spec.LocalDiskManager = &hwameistoriov1alpha1.LocalDiskManagerSpec{
		KubeletRootDir: "/var/lib/kubelet",
		Common: &hwameistoriov1alpha1.PodCommonSpec{
			Tolerations: &[]corev1.Toleration{{
				Key:      "dedicated",
				Operator: corev1.TolerationOpEqual,
				Value:    "storage",
				Effect:   corev1.TaintEffectNoSchedule,
			}},
		},
		Manager: &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
		CSI: &hwameistoriov1alpha1.CSISpec{
			Registrar: &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
			Controller: &hwameistoriov1alpha1.CSIControllerSpec{
				Provisioner: &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
				Attacher:    &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
			},
		},
	}

	daemonSet := SetLDMDaemonSet(cluster)
	if len(daemonSet.Spec.Template.Spec.Tolerations) != 1 {
		t.Fatalf("expected 1 toleration, got %d", len(daemonSet.Spec.Template.Spec.Tolerations))
	}
	if daemonSet.Spec.Template.Spec.Tolerations[0].Key != "dedicated" {
		t.Fatalf("expected custom toleration to be propagated, got %#v", daemonSet.Spec.Template.Spec.Tolerations)
	}
}

func TestSetLDMDaemonSetAppendsMasterTolerations(t *testing.T) {
	cluster := &hwameistoriov1alpha1.Cluster{}
	cluster.Spec.TargetNamespace = "hwameistor"
	cluster.Spec.RBAC = &hwameistoriov1alpha1.RBACSpec{ServiceAccountName: "hwameistor-admin"}
	cluster.Spec.LocalDiskManager = &hwameistoriov1alpha1.LocalDiskManagerSpec{
		KubeletRootDir:     "/var/lib/kubelet",
		TolerationOnMaster: true,
		Common: &hwameistoriov1alpha1.PodCommonSpec{
			Tolerations: &[]corev1.Toleration{{
				Key:      "dedicated",
				Operator: corev1.TolerationOpEqual,
				Value:    "storage",
				Effect:   corev1.TaintEffectNoSchedule,
			}},
		},
		Manager: &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
		CSI: &hwameistoriov1alpha1.CSISpec{
			Registrar: &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
			Controller: &hwameistoriov1alpha1.CSIControllerSpec{
				Provisioner: &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
				Attacher:    &hwameistoriov1alpha1.ContainerCommonSpec{Image: &hwameistoriov1alpha1.ImageSpec{}},
			},
		},
	}

	daemonSet := SetLDMDaemonSet(cluster)
	if len(daemonSet.Spec.Template.Spec.Tolerations) <= 1 {
		t.Fatalf("expected custom + master tolerations, got %#v", daemonSet.Spec.Template.Spec.Tolerations)
	}
	if daemonSet.Spec.Template.Spec.Tolerations[0].Key != "dedicated" {
		t.Fatalf("expected custom toleration first, got %#v", daemonSet.Spec.Template.Spec.Tolerations)
	}
}
