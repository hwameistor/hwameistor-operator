FROM centos:7

RUN yum upgrade nss -y

COPY ./vendor/github.com/hwameistor/hwameistor/deploy/crds /hwameistorcrds
COPY ./vendor/github.com/hwameistor/datastore/deploy/crds /hwameistorcrds
COPY ./scheduler-config.yaml /scheduler-config.yaml

COPY ./_build/operator /operator

ENTRYPOINT [ "/operator" ]
