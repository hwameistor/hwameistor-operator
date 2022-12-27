package evictor

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

var evictorDeployment = appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-volume-evictor",
		Labels: map[string]string{
			"app": "hwameistor-volume-evictor",
		},
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &replicas,
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "hwameistor-volume-evictor",
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app" : "hwameistor-volume-evictor",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "evictor",
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
			},
		},
	},
}

func SetEvictor(clusterInstance *hwameistoriov1alpha1.Cluster) {
	evictorDeployment.Namespace = clusterInstance.Spec.TargetNamespace
	evictorDeployment.Spec.Template.Spec.ServiceAccountName = clusterInstance.Spec.RBAC.ServiceAccountName
	setEvictorContainers(clusterInstance)
}

func setEvictorContainers(clusterInstance *hwameistoriov1alpha1.Cluster) {
	for i, container := range evictorDeployment.Spec.Template.Spec.Containers {
		if container.Name == "evictor" {
			// container.Resources = *clusterInstance.Spec.Evictor.Evictor.Resources
			imageSpec := clusterInstance.Spec.Evictor.Evictor.Image
			container.Image = imageSpec.Registry + "/" + imageSpec.Repository + ":" + imageSpec.Tag
		}
		evictorDeployment.Spec.Template.Spec.Containers[i] = container
	}
}

func InstallEvictor(cli client.Client) error {
	if err := cli.Create(context.TODO(), &evictorDeployment); err != nil {
		log.Errorf("Create Evictor err: %v", err)
		return err
	}

	return nil
}