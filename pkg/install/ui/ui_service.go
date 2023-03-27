package ui

import (
	"context"

	operatorv1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UIServiceMaintainer struct {
	Client client.Client
	ClusterInstance *operatorv1alpha1.Cluster
}

func NewUIServiceMaintainer(cli client.Client, clusterInstance *operatorv1alpha1.Cluster) *UIServiceMaintainer {
	return &UIServiceMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var uiService = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-ui",
		Labels: map[string]string{
			uiLabelKey: uiLabelValue,
		},
	},
	Spec: corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Port: 80,
				Protocol: corev1.ProtocolTCP,
				TargetPort: intstr.IntOrString{
					Type: intstr.Int,
					IntVal: 80,
				},
			},
		},
		Selector: map[string]string{
			uiLabelKey: uiLabelValue,
		},
	},
}

func SetUIService(clusterInstance *operatorv1alpha1.Cluster) {
	uiService.Namespace = clusterInstance.Spec.TargetNamespace
}

func (m *UIServiceMaintainer) Ensure() error {
	SetUIService(m.ClusterInstance)
	key := types.NamespacedName{
		Namespace: uiService.Namespace,
		Name: uiService.Name,
	}
	var gotten corev1.Service
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &uiService); errCreate != nil {
				log.Errorf("Create UI Service err: %v", err)
				return errCreate
			}
		} else {
			log.Errorf("Get UI Service err: %v", err)
			return err
		}
	}

	return nil
}