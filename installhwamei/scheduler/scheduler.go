package scheduler

import (
	"context"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var schedulerDeploy = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-scheduler",
	},
	Spec: appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "hwameistor-scheduler",
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "hwameistor-scheduler",
				},
			},
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
							{
								Weight: 1,
								Preference: corev1.NodeSelectorTerm{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key: "node-role.kubernetes.io/master",
											Operator: corev1.NodeSelectorOpExists,
										},
										{
											Key: "node-role.kubernetes.io/control-plane",
											Operator: corev1.NodeSelectorOpExists,
										},
									},
								},
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: "hwameistor-kube-scheduler",
						Args: []string{
							"-v=2",
							"--bind-address=0.0.0.0",
							"--leader-elect=false",
							"--leader-elect-resource-name=hwameistor-scheduler",
							"--config=/etc/hwameistor/hwameistor-scheduler-config.yaml",
						},
						ImagePullPolicy: "IfNotPresent",
						TerminationMessagePath: "/dev/termination-log",
						TerminationMessagePolicy: "File",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "hwameistor-scheduler-config",
								MountPath: "/etc/hwameistor/",
								ReadOnly: true,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "hwameistor-scheduler-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								Items: []corev1.KeyToPath{
									{
										Key: "hwameistor-scheduler-config.yaml",
										Path: "hwameistor-scheduler-config.yaml",
									},
								},
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "hwameistor-scheduler-config",
								},
							},
						},
					},
				},
				Tolerations: []corev1.Toleration{
					{
						Key: "CriticalAddonsOnly",
						Operator: corev1.TolerationOpExists,
					},
					{
						Key: "node.kubernetes.io/not-ready",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key: "node-role.kubernetes.io/master",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key: "node-role.kubernetes.io/control-plane",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key: "node.cloudprovider.kubernetes.io/uninitialized",
						Operator: corev1.TolerationOpExists,
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
			},
		},
	},
}

func SetScheduler(clusterInstance *hwameistoriov1alpha1.Cluster) {
	schedulerDeploy.Namespace = clusterInstance.Spec.TargetNamespace
	schedulerDeploy.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	replicas := clusterInstance.Spec.Scheduler.Replicas
	schedulerDeploy.Spec.Replicas = &replicas
	setSchedulerContainers(clusterInstance)
}

func setSchedulerContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range schedulerDeploy.Spec.Template.Spec.Containers {
		if container.Name == "hwameistor-kube-scheduler" {
			// container.Resources = *clusterInstance.Spec.Scheduler.Scheduler.Resources
			imageSpec := clusterInstance.Spec.Scheduler.Scheduler.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
		}
		schedulerDeploy.Spec.Template.Spec.Containers[i] = container
	}
}

func InstallScheduler(cli client.Client) error {
	if err := cli.Create(context.TODO(), &schedulerDeploy); err != nil {
		log.Errorf("Create Scheduler err: %v", err)
		return err
	}

	return nil
}