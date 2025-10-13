package install

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"strings"

	hwameistoriov1alpha1 "github.com/hwameistor/hwameistor-operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	syaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var defaultTargetNamespace = "hwameistor"

func EnsureTargetNamespaceExist(cli client.Client, targetNamespace string) (bool, error) {
	var reReconcile bool
	key := types.NamespacedName{
		Name: targetNamespace,
	}
	ns := corev1.Namespace{}
	if err := cli.Get(context.TODO(), key, &ns); err == nil {
		return reReconcile, nil
	} else if errors.IsNotFound(err) {
		ns.Name = targetNamespace
		if createErr := cli.Create(context.TODO(), &ns); createErr != nil {
			log.Errorf("Create namespace %v err", ns.Name)
			return reReconcile, createErr
		}
		reReconcile = true
		return reReconcile, nil
	} else {
		return reReconcile, err
	}
}

func ReadResourcesFromDir(dir string) ([][]byte, error) {
	var contents [][]byte
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Errorf("read dir err: %v\n", err)
		return nil, err
	}

	for _, fileInfo := range fileInfos {
		if !strings.HasSuffix(fileInfo.Name(), ".yaml") {
			continue
		}
		content, err := GetFileBytes(dir + "/" + fileInfo.Name())
		if err != nil {
			log.Errorf("get file bytes err: %v\n", err)
			return nil, err
		}
		contents = append(contents, content)
	}

	return contents, nil
}

func GetFileBytes(filepath string) ([]byte, error) {
	content, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Errorf("read file err: %v\n", err)
		return nil, err
	}
	return content, nil
}

func Install(cli client.Client, resourceBytes []byte, targetNamespace string) error {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(resourceBytes), len(resourceBytes))
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

		// obj, _, err := syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		// if err != nil {
		// log.Errorf("decode err: %v", err)
		// 	return err
		// }

		// unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		// if err != nil {
		// log.Errorf("convert err: %v", err)
		// 	return err
		// }

		// unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}

		unstructuredObj, err := RawExtensionToUnstructured(rawObj)
		if err != nil {
			log.Errorf("RawExtensionToUnstructured err: %v", err)
			return err
		}

		if unstructuredObj.GetKind() == "ConfigMap" {
			unstructuredObj.SetNamespace(targetNamespace)
		}

		if err := cli.Create(context.TODO(), unstructuredObj); err != nil {
			log.Errorf("create err: %v", err)
			return err
		}
	}

	return nil
}

func RawExtensionToUnstructured(rawObj runtime.RawExtension) (*unstructured.Unstructured, error) {
	obj, _, err := syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
	if err != nil {
		log.Errorf("decode err: %v", err)
		return nil, err
	}

	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		log.Errorf("convert err: %v", err)
		return nil, err
	}

	unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}
	return unstructuredObj, nil
}

func FulfillTargetNamespaceSpec(clusterInstance *hwameistoriov1alpha1.Cluster) *hwameistoriov1alpha1.Cluster {
	if clusterInstance.Spec.TargetNamespace == "" {
		clusterInstance.Spec.TargetNamespace = defaultTargetNamespace
	}

	return clusterInstance
}

// Contains checks if a string slice contains a specific string
func Contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}