FROM centos:7

COPY ./resourcestoinstall /resourcestoinstall

COPY ./_build/operator /operator

ENTRYPOINT [ "/operator" ]
