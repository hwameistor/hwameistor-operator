#! /usr/bin/env bash

set -x
set -e
# git clone https://github.com/hwameistor/hwameistor.git test/hwameistor

# common defines
date=$(date +%Y%m%d%H%M)
IMAGE_TAG=v${date}
export IMAGE_TAG=${IMAGE_TAG}
OPERATOR_MODULE_NAME=operator
IMAGE_REGISTRY=172.30.45.210/hwameistor
export IMAGE_NAME=${IMAGE_REGISTRY}/${OPERATOR_MODULE_NAME}
MODULES=(operator)

function build_image(){
	echo "Build hwameistor image"
	export IMAGE_TAG=${IMAGE_TAG} && make build_image

	for module in ${MODULES[@]}
	do
		docker push ${IMAGE_REGISTRY}/${module}:${IMAGE_TAG}
	done
}

function prepare_install_params() {
	# FIXME: image tags should be passed by helm install params
		# sed -i '/.*ghcr.io*/c\ \ hwameistorImageRegistry: '$ImageRegistry'' helm/operator/values.yaml
  	sed -i '/operator:/a\ \ imageRegistry: '$ImageRegistry'' helm/operator/values.yaml
  	sed -i ' s/.Values.global.hwameistorImageRegistry/.Values.operator.imageRegistry/' helm/operator/templates/deployment.yaml
   IMAGE_VERSION=$(sed -n '/version:/p' helm/operator/Chart.yaml)
   export IMAGE_VERSION=${IMAGE_VERSION}
   sed -i "s/${IMAGE_VERSION}/version: ${IMAGE_TAG}/" helm/operator/Chart.yaml

}

# Step1: build all images tagged with <image_registry>/<module>:<date>
timer_start=`date "+%Y-%m-%d %H:%M:%S"`
build_image
timer_end=`date "+%Y-%m-%d %H:%M:%S"`

prepare_install_params
# Step3: go e2e test
ginkgo -timeout=10h --fail-fast  --label-filter=${E2E_TESTING_LEVEL} test/e2e

