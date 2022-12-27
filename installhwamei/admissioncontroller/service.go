package admissioncontroller

import (
	"context"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var admissionControllerService = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-admission-controller",
	},
	Spec: corev1.ServiceSpec{
		Selector: map[string]string{
			"app": "hwameistor-admission-controller",
		},
		Ports: []corev1.ServicePort{
			{
				Port: 443,
				TargetPort: intstr.IntOrString{
					Type: intstr.String,
					StrVal: "admission-api",
				},
			},
		},
	},
}

func SetAdmissionControllerService(clusterInstance *hwameistoriov1alpha1.Cluster) {
	admissionControllerService.Namespace = clusterInstance.Spec.TargetNamespace
}

func InstallAdmissionControllerService(cli client.Client) error {
	if err := cli.Create(context.TODO(), &admissionControllerService); err != nil {
		log.Errorf("Create Admission Controller Service err: %v", err)
		return err
	}

	return nil
}