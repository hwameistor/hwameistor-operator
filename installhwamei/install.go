package installhwamei

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	syaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Install(cli client.Client) error {
  dir, set := os.LookupEnv("ResourcesDir")
  if !set {
    dir = "/resourcestoinstall"
  }

  resources, err := readResourcesFromDir(dir)
  if err != nil {
    log.Errorf("Read resources err: %v", err)
    return err
  }

  for _, resource := range resources {
    if err := install(cli, resource); err != nil {
      log.Errorf("Install err: %v", err)
      return err
    }
  }

  return nil
}

func install(cli client.Client, resourceBytes []byte) error {
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

		if err := cli.Create(context.TODO(), unstructuredObj); err != nil {
      log.Errorf("create err: %v", err)
			return err
		}
	}

	return nil
}

func readResourcesFromDir(dir string) ([][]byte, error) {
  var contents [][]byte
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Errorf("read dir err: %v\n", err)
		return nil, err
	}

	for _, fileInfo := range fileInfos {
		content, err := getFileBytes(dir + "/" + fileInfo.Name())
		if err != nil {
			log.Errorf("get file bytes err: %v\n", err)
			return nil, err
		}
		contents = append(contents, content)
	}

	return contents, nil

}

func getFileBytes(filepath string) ([]byte, error) {
	content, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Errorf("read file err: %v\n", err)
		return nil, err
	}
	return content, nil
}
