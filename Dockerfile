FROM rockylinux:8

RUN yum -y --refresh upgrade ca-certificates 'glibc*' \
    && yum install -y nss \
    && glibc_version=$(rpm -q --qf '%{VERSION}-%{RELEASE}' glibc) \
    && ca_version=$(rpm -q --qf '%{VERSION}-%{RELEASE}' ca-certificates) \
    && glibc_check="%{lua:print(rpm.vercmp('$glibc_version', '2.28-251.el8_10.11'))}" \
    && ca_check="%{lua:print(rpm.vercmp('$ca_version', '2026.2.92_v9.0.319-81.3.el8_10'))}" \
    && test "$(rpm --eval "$glibc_check")" -ge 0 \
    && test "$(rpm --eval "$ca_check")" -ge 0 \
    && yum clean all

COPY ./vendor/github.com/hwameistor/hwameistor/deploy/crds /hwameistorcrds
COPY ./vendor/github.com/hwameistor/datastore/deploy/crds /hwameistorcrds
COPY ./config/snapshot-crds/ /hwameistorcrds/
COPY ./scheduler-config.yaml /scheduler-config.yaml

COPY ./_build/operator /operator

ENTRYPOINT [ "/operator" ]
