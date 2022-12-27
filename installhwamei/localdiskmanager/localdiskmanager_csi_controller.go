package localdiskmanager

import (
	"context"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	installhwamei "github.com/hwameistor/hwameistor-operator/installhwamei"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ldmCSIControllerReplicas = int32(1)

var ldmCSIController = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-local-disk-csi-controller",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &ldmCSIControllerReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "hwameistor-local-disk-csi-controller",
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "hwameistor-local-disk-csi-controller",
				},
			},
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key: "app",
											Operator: "In",
											Values: []string{"hwameistor-local-disk-manager"},
										},
									},
								},
								TopologyKey: "topology.disk.hwameistor.io/node",
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: "provisioner",
						ImagePullPolicy: "IfNotPresent",
						Args: []string{
							"--v=5",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election=true",
							"--feature-gates=Topology=true",
							"--strict-topology",
							"--extra-create-metadata=true",
						},
						Env: []corev1.EnvVar{
							{
								Name: "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "socket-dir",
								MountPath: "/csi",
							},
						},
					},
					{
						Name: "attacher",
						ImagePullPolicy: "IfNotPresent",
						Args: []string{
							"--v=5",
							"--csi-address=$(CSI_ADDRESS)",
							"--leader-election=true",
							"--timeout=120s",
						},
						Env: []corev1.EnvVar{
							{
								Name: "CSI_ADDRESS",
								Value: "/csi/csi.sock",
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "socket-dir",
								MountPath: "/csi",
							},
						},
					},
				},
			},
		},
	},
}

func SetLDMCSIController(clusterInstance *hwameistoriov1alpha1.Cluster) {
	ldmCSIController.Namespace = clusterInstance.Spec.TargetNamespace
	// ldmCSIController.Spec.Template.Spec.PriorityClassName = clusterInstance.Spec.LocalDiskManager.CSI.Controller.Common.PriorityClassName
	ldmCSIController.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	setLDMCSIControllerVolumes(clusterInstance)
	setLDMCSIControllerContainers(clusterInstance)
}

func setLDMCSIControllerVolumes(clusterInstance *hwameistoriov1alpha1.Cluster) {
	volume := corev1.Volume{
		Name: "socket-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Type: &installhwamei.HostPathDirectoryOrCreate,
				Path: clusterInstance.Spec.LocalDiskManager.KubeletRootDir + "/plugins/disk.hwameistor.io",
			},
		},
	}

	ldmCSIController.Spec.Template.Spec.Volumes = append(ldmCSIController.Spec.Template.Spec.Volumes, volume)
}

func setLDMCSIControllerContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range ldmCSIController.Spec.Template.Spec.Containers {
		if container.Name == "provisioner" {
			// container.Resources = *clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Resources
			imageSpec := clusterInstance.Spec.LocalDiskManager.CSI.Controller.Provisioner.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
		}
		if container.Name == "attacher" {
			// container.Resources = *clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Resources
			imageSpec := clusterInstance.Spec.LocalDiskManager.CSI.Controller.Attacher.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
		}

		ldmCSIController.Spec.Template.Spec.Containers[i] = container
	}
}

func InstallLDMCSIController(cli client.Client) error {
	if err := cli.Create(context.TODO(), &ldmCSIController); err != nil {
		log.Errorf("Create LocalDiskManager CSI Controller err: %v", err)
		return err
	}

	return nil
}