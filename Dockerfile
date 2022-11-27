FROM centos:7

COPY ./resourcestoinstall /resourcestoinstall

COPY ./_build/hwameistor-operator /hwameistor-operator

ENTRYPOINT [ "/hwameistor-operator" ]
