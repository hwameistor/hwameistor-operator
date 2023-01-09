package apiserver

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

type ApiServerServiceMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewApiServerServiceMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *ApiServerServiceMaintainer {
	return &ApiServerServiceMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var apiServerService = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-apiserver",
	},
	Spec: corev1.ServiceSpec{
		Selector: map[string]string{
			apiServerLabelSelectorKey: apiServerLabelSelectorValue,
		},
		Ports: []corev1.ServicePort{
			{
				TargetPort: intstr.IntOrString{
					Type: intstr.String,
					StrVal: "http",
				},
				Port: 80,
			},
		},
	},
}

func SetApiServerService(clusterInstance *hwameistoriov1alpha1.Cluster) {
	apiServerService.Namespace = clusterInstance.Spec.TargetNamespace
}

func (m *ApiServerServiceMaintainer) Ensure() error {
	SetApiServerService(m.ClusterInstance)
	key := types.NamespacedName{
		Namespace: apiServerService.Namespace,
		Name: apiServerService.Name,
	}
	var gottenService corev1.Service
	if err := m.Client.Get(context.TODO(), key, &gottenService); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &apiServerService); errCreate != nil {
				log.Errorf("Create ApiServer Service err: %v", err)
				return errCreate
			}
		} else {
			log.Errorf("Get ApiServer Service err: %v", err)
			return err
		}
	}

	return nil
}