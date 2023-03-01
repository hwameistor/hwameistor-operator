FROM centos:7

COPY ./vendor/github.com/hwameistor/hwameistor/deploy/crds /hwameistorcrds
COPY ./scheduler-config.yaml /scheduler-config.yaml

COPY ./_build/operator /operator

ENTRYPOINT [ "/operator" ]
