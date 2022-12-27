package admissioncontroller

import (
	"context"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var replicas = int32(1)

var admissionController = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-admission-controller",
		Labels: map[string]string{
			"app": "hwameistor-admission-controller",
		},
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &replicas,
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "hwameistor-admission-controller",
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "hwameistor-admission-controller",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "server",
						Args: []string{
							"--cert-dir=/etc/webhook/certs",
							"--tls-private-key-file=tls.key",
							"--tls-cert-file=tls.crt",
						},
						ImagePullPolicy: "IfNotPresent",
						Env: []corev1.EnvVar{
							{
								Name: "MUTATE_CONFIG",
								Value: "hwameistor-admission-mutate",
							},
							{
								Name: "WEBHOOK_SERVICE",
								Value: "hwameistor-admission-controller",
							},
							{
								Name: "MUTATE_PATH",
								Value: "/mutate",
							},
						},
						Ports: []corev1.ContainerPort{
							{
								Name: "admission-api",
								ContainerPort: 18443,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "admission-controller-tls-certs",
								MountPath: "/etc/webhook/certs",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "admission-controller-tls-certs",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		},
	},
}

func SetAdmissionController(clusterInstance *hwameistoriov1alpha1.Cluster) {
	admissionController.Namespace = clusterInstance.Spec.TargetNamespace
	admissionController.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	setAdmissionControllerContainers(clusterInstance)
}

func setAdmissionControllerContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range admissionController.Spec.Template.Spec.Containers {
		if container.Name == "server" {
			// container.Resources = *clusterInstance.Spec.AdmissionController.Controller.Resources
			imageSpec := clusterInstance.Spec.AdmissionController.Controller.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
			container.Env = append(container.Env, []corev1.EnvVar{
				{
					Name: "WEBHOOK_NAMESPACE",
					Value: clusterInstance.Spec.TargetNamespace,
				},
				{
					Name: "FAILURE_POLICY",
					Value: clusterInstance.Spec.AdmissionController.FailurePolicy,
				},
			}...)
		}
		admissionController.Spec.Template.Spec.Containers[i] = container
	}
}

func InstallAdmissionController(cli client.Client) error {
	if err := cli.Create(context.TODO(), &admissionController); err != nil {
		log.Errorf("Create Admission Controller err: %v", err)
		return err
	}

	return nil
}