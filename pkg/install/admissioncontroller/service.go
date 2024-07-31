package admissioncontroller

import (
	"context"
	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AdmissionControllerServiceMaintainer struct {
	Client          client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewAdmissionControllerServiceMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *AdmissionControllerServiceMaintainer {
	return &AdmissionControllerServiceMaintainer{
		Client:          cli,
		ClusterInstance: clusterInstance,
	}
}

var admissionControllerService = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-admission-controller",
	},
	Spec: corev1.ServiceSpec{
		Selector: map[string]string{
			admissionControllerLabelSelectorKey: admissionControllerLabelSelectorValue,
		},
		Ports: []corev1.ServicePort{
			{
				Port: 443,
				TargetPort: intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "admission-api",
				},
			},
		},
	},
}

func SetAdmissionControllerService(clusterInstance *hwameistoriov1alpha1.Cluster) {
	admissionControllerService.Namespace = clusterInstance.Spec.TargetNamespace
}

func (m *AdmissionControllerServiceMaintainer) Ensure() error {
	SetAdmissionControllerService(m.ClusterInstance)
	key := types.NamespacedName{
		Namespace: admissionControllerService.Namespace,
		Name:      admissionController.Name,
	}
	var gottenService corev1.Service
	if err := m.Client.Get(context.TODO(), key, &gottenService); err != nil {
		if errors.IsNotFound(err) {
			resourceCreate := admissionControllerService.DeepCopy()
			if errCreate := m.Client.Create(context.TODO(), resourceCreate); errCreate != nil {
				log.Errorf("Create AdmissionController Service err: %v", err)
				return errCreate
			}
		} else {
			log.Errorf("Get AdmissionController Service err: %v", err)
			return err
		}
	}

	return nil
}

func (m *AdmissionControllerServiceMaintainer) Uninstall() error {
	key := types.NamespacedName{
		Namespace: m.ClusterInstance.Spec.TargetNamespace,
		Name:      admissionController.Name,
	}
	var gottenService corev1.Service
	if err := m.Client.Get(context.TODO(), key, &gottenService); err != nil {
		if errors.IsNotFound(err) {
			log.WithError(err)
			return nil
		} else {
			log.Errorf("Get AdmissionController Service err: %v", err)
			return err
		}
	} else {
		if err = m.Client.Delete(context.TODO(), &gottenService); err != nil {
			return err
		}
	}

	return nil
}
