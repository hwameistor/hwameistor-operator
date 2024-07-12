FROM centos:7

FROM centos:7

# Update the CentOS repository configuration
RUN sed -i 's|mirrorlist=http://mirrorlist.centos.org/?|#mirrorlist=http://mirrorlist.centos.org/?|g' /etc/yum.repos.d/CentOS-*.repo && \
    sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*.repo

# Install EPEL repository and upgrade nss
RUN yum install -y epel-release && \
    yum-config-manager --enable epel && \
    yum --disablerepo=base upgrade nss -y


COPY ./vendor/github.com/hwameistor/hwameistor/deploy/crds /hwameistorcrds
COPY ./vendor/github.com/hwameistor/datastore/deploy/crds /hwameistorcrds
COPY ./scheduler-config.yaml /scheduler-config.yaml

COPY ./_build/operator /operator

ENTRYPOINT [ "/operator" ]
