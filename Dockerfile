FROM centos:7

COPY ./hwameistorcrds /hwameistorcrds
COPY ./scheduler-config.yaml /scheduler-config.yaml

COPY ./_build/operator /operator

ENTRYPOINT [ "/operator" ]
