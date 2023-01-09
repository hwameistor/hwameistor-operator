package metrics

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

type MetricsServiceMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewMetricsServiceMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *MetricsServiceMaintainer {
	return &MetricsServiceMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

var metricsService = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name: "hwameistor-metrics-collector",
	},
	Spec: corev1.ServiceSpec{
		Selector: map[string]string{
			metricsLabelSelectorKey: metricsLabelSelectorValue,
		},
		Ports: []corev1.ServicePort{
			{
				TargetPort: intstr.IntOrString{
					Type: intstr.String,
					StrVal: "metrics-apis",
				},
				Port: 8080,
			},
		},
	},
}

func SetMetricsService(clusterInstance *hwameistoriov1alpha1.Cluster) {
	metricsService.Namespace = clusterInstance.Spec.TargetNamespace
}

func (m *MetricsServiceMaintainer) Ensure() error {
	SetMetricsService(m.ClusterInstance)
	key := types.NamespacedName{
		Namespace: metricsService.Namespace,
		Name: metricsService.Name,
	}
	var gottenService corev1.Service
	if err := m.Client.Get(context.TODO(), key, &gottenService); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), &metricsService); errCreate != nil {
				log.Errorf("Create Metrics Service err: %v", err)
				return errCreate
			}
		} else {
			log.Errorf("Get Metrics Service err: %v", err)
			return err
		}
	}

	return nil
}