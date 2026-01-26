# Copyright 2021-2026 Andreas Gebhardt
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.10-1769387960 AS builder
WORKDIR /go/src/github/agebhar1/mq_exporter
RUN microdnf install tar gzip findutils gcc make git \
    && mkdir /opt/mqm && curl -L https://ibm.biz/IBM-MQC-Redist-LinuxX64targz | tar xzf - -C /opt/mqm/ \
    && curl -L https://go.dev/dl/go1.25.6.linux-amd64.tar.gz | tar xzf - -C /opt

COPY . ./
RUN PATH=$PATH:/opt/go/bin make

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.10-1769387960
WORKDIR /opt/mq_exporter
COPY --from=builder /go/src/github/agebhar1/mq_exporter/mq_exporter ./

WORKDIR /tmp
# prevent for Docker "AMQ6300E: Directory '/IBM' could not be created: 'EACCES - Permission denied'"
ENV HOME=/tmp

LABEL \
    description="Prometheus exporter for IBM MQ" \
    maintainer="Andreas Gebhardt <agebhar1@googlemail.com>" \
    name="mq_exporter" \
    summary="" \
    url="https://github.com/agebhar1/mq_exporter" \
    vendor="Andreas Gebhardt" \
    release="1" \
    vcs-type="git" \
    io.k8s.description="" \
    io.k8s.display-name="" \
    org.opencontainers.image.authors="Andreas Gebhardt <agebhar1@googlemail.com>" \
    org.opencontainers.image.url="https://github.com/agebhar1/mq_exporter" \
    org.opencontainers.image.documentation="https://github.com/agebhar1/mq_exporter" \
    org.opencontainers.image.source="https://github.com/agebhar1/mq_exporter" \
    org.opencontainers.image.vendor="Andreas Gebhardt" \
    org.opencontainers.image.licenses="Apache-2.0" \
    org.opencontainers.image.title="mq_exporter" \
    org.opencontainers.image.description="Prometheus exporter for IBM MQ" \
    org.opencontainers.image.base.name="registry.access.redhat.com/ubi8/ubi-minimal:8.10-1769387960"

EXPOSE 9873

# nobody
USER 65534

CMD [ "/opt/mq_exporter/mq_exporter", "--config", "/etc/mq_exporter/config.yml" ]
