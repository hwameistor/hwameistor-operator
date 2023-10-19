package scheduler

import (
	"bytes"
	"context"
	"io"
	"os"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	"github.com/hwameistor/hwameistor-operator/pkg/install"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SchedulerConfigMapMaintainer struct {
	Client client.Client
	ClusterInstance *hwameistoriov1alpha1.Cluster
}

func NewSchedulerConfigMapMaintainer(cli client.Client, clusterInstance *hwameistoriov1alpha1.Cluster) *SchedulerConfigMapMaintainer {
	return &SchedulerConfigMapMaintainer{
		Client: cli,
		ClusterInstance: clusterInstance,
	}
}

func InstallSchedulerConfigMap(cli client.Client, targetNamespace string) error {
	filePath, set := os.LookupEnv("SchedulerConfigMap")
	if !set {
		filePath = "/scheduler-config.yaml"
	}

	fileBytes, err := install.GetFileBytes(filePath)
	if err != nil {
		log.Errorf("GetFileBytes err: %v", err)
		return err
	}

	if err := install.Install(cli, fileBytes, targetNamespace); err != nil {
		log.Errorf("Create scheduler configmap err: %v", err)
		return err
	}

	return nil
}

func (m *SchedulerConfigMapMaintainer) Ensure() error {
	filePath, set := os.LookupEnv("SchedulerConfigMap")
	if !set {
		filePath = "/scheduler-config.yaml"
	}

	fileBytes, err := install.GetFileBytes(filePath)
	if err != nil {
		log.Errorf("GetFileBytes err: %v", err)
		return err
	}

	var cmUnstructured *unstructured.Unstructured

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(fileBytes), len(fileBytes))
	for {
		var rawObj runtime.RawExtension
		err := decoder.Decode(&rawObj)
		if err != nil {
			if err == io.EOF {
				break
			} else {
        log.Errorf("decode err: %v", err)
				return err
			}
		}

		cmUnstructured, err = install.RawExtensionToUnstructured(rawObj)
		if err != nil {
			log.Errorf("RawExtensionToUnstructured err: %v", err)
			return err
		}

		if cmUnstructured.GetKind() == "ConfigMap" {
			cmUnstructured.SetNamespace(m.ClusterInstance.Spec.TargetNamespace)
		}
	}

	key := types.NamespacedName{
		Namespace: cmUnstructured.GetNamespace(),
		Name: cmUnstructured.GetName(),
	}
	var gotten corev1.ConfigMap
	if err := m.Client.Get(context.TODO(), key, &gotten); err != nil {
		if errors.IsNotFound(err) {
			if errCreate := m.Client.Create(context.TODO(), cmUnstructured); errCreate != nil {
				log.Errorf("Create Scheduler ConfigMap err: %v", errCreate)
				return errCreate
			}
		} else {
			log.Errorf("Get Scheduler ConfigMap err: %v", err)
			return err
		}
	} else {
		wantedData, exist, err := unstructured.NestedMap(cmUnstructured.Object, "data")
		if err != nil {
			log.Errorf("get nested map from unstructured configmap err: ", err)
			return err
		}
		if !exist {
			log.Infof("not found data map in unstructured configmap")
			return nil
		}
		for k, v := range wantedData {
			s := v.(string)
			if gotten.Data[k] != s {
				log.Infof("scheduler configmap not the same as expected, update it")
				if errUpdate := m.Client.Update(context.TODO(), cmUnstructured); errUpdate != nil {
					log.Errorf("update scheduler configmap err: %v", errUpdate)
					return errUpdate
				}
			}
		} 
	}

	return nil
}