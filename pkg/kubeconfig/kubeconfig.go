package kubeconfig

import "k8s.io/client-go/rest"

var kubeConfig *rest.Config

func Set(config *rest.Config) {
	kubeConfig = config
}

func Get() *rest.Config {
	return kubeConfig
}
