package installhwamei

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	syaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EnsureTargetNamespaceExist(cli client.Client, targetNamespace string) error {
	key := types.NamespacedName{
		Name: targetNamespace,
	}
	ns := corev1.Namespace{}
	if err := cli.Get(context.TODO(), key, &ns) ; err == nil {
		return nil
	} else if errors.IsNotFound(err) {
		ns.Name = targetNamespace
		if createErr := cli.Create(context.TODO(), &ns); createErr != nil {
			log.Errorf("Create namespace %v err", ns.Name)
			return createErr
		}

		return nil
	} else {
		return err
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

		obj, _, err := syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
      	log.Errorf("decode err: %v", err)
			return err
		}

		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
      	log.Errorf("convert err: %v", err)
			return err
		}

		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}

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
  