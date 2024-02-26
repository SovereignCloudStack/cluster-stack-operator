# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This is the Dockerfile for the Builder Image that is used by the CI
# pipelines.
# If you make changes to this Dockerfile run `make builder-image-push`.

# Install Lychee
FROM docker.io/library/alpine:3.19.0@sha256:13b7e62e8df80264dbb747995705a986aa530415763a6c58f84a3ca8af9a5bcd as lychee
# hadolint ignore=DL3018
# update: datasource=github-tags depName=lycheeverse/lychee versioning=semver
ENV LYCHEE_VERSION="v0.14.3"
RUN apk add --no-cache curl && \
    curl -L -o /tmp/lychee-${LYCHEE_VERSION}.tgz https://github.com/lycheeverse/lychee/releases/download/${LYCHEE_VERSION}/lychee-${LYCHEE_VERSION}-x86_64-unknown-linux-gnu.tar.gz && \
    tar -xz -C /tmp -f /tmp/lychee-${LYCHEE_VERSION}.tgz && \
    mv /tmp/lychee /usr/bin/lychee && \
    rm -rf /tmp/linux-amd64 /tmp/lychee-${LYCHEE_VERSION}.tgz

FROM cgr.dev/chainguard/wolfi-base:latest as wolfi

# renovate: datasource=github-tags depName=kubernetes-sigs/cluster-api
ARG CLUSTERCTL_VERSION="v1.6.2"
# renovate: datasource=github-tags depName=helm/helm
ENV HELM_VERSION="v3.14.1"
# renovate: datasource=github-tags depName=kubernetes-sigs/kind
ARG KIND_VERSION="v0.20.0"
# renovate: datasource=github-tags depName=kubernetes/kubernetes
ARG KUBECTL_VERSION="v1.27.3"
# renovate: datasource=github-tags depName=kubernetes-sigs/kustomize extractVersion=^kustomize\/v(?<version>.+)$
ARG KUSTOMIZE_VERSION="v5.3.0"
# renovate: datasource=github-tags depName=aquasecurity/trivy
ARG TRIVY_VERSION="v0.48.3"
# renovate: datasource=github-tags depName=containers/skopeo
ARG SKOPEO_VERSION="v1.14.1"
# renovate: datasource=github-tags depName=kubernetes-sigs/controller-tools
ARG CONTROLLER_GEN_VERSION="v0.14.0"

# hadolint ignore=DL3018
RUN apk add -U --no-cache \
    curl \
    clusterctl=~${CLUSTERCTL_VERSION#v} \
    controller-gen=~${CONTROLLER_GEN_VERSION#v} \
    helm=~${HELM_VERSION#v} \
    kind=~${KIND_VERSION#v} \
    kubectl=~${KUBECTL_VERSION#v} \
    kustomize=~${KUSTOMIZE_VERSION#v} \
    skopeo=~${SKOPEO_VERSION#v} \
    trivy=~${TRIVY_VERSION#v} 

# Install Golang CI Lint
FROM docker.io/library/alpine:3.19.1 as golangci
# update: datasource=github-tags depName=golangci/golangci-lint versioning=semver
ENV GOLANGCI_VERSION="v1.55.2"
WORKDIR /
# hadolint ignore=DL3018,DL4006
RUN apk add --no-cache curl && \
    curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s ${GOLANGCI_VERSION}

# Install Hadolint
FROM docker.io/hadolint/hadolint:v2.12.0-alpine@sha256:7dba9a9f1a0350f6d021fb2f6f88900998a4fb0aaf8e4330aa8c38544f04db42 as hadolint

############################
# CSO Build Image Base #
############################
FROM docker.io/library/golang:1.21.6-bullseye

# update: datasource=github-tags depName=adrienverge/yamllint versioning=semver
ENV YAMLLINT_VERSION="v1.33.0"
# update: datasource=github-tags depName=opt-nc/yamlfixer versioning=semver
ENV YAMLFIXER_VERSION="0.9.15"

# hadolint ignore=DL3008
RUN apt-get update && \
    apt-get install -qy --no-install-recommends \
    gnupg python3 python3-pip \
    file zip unzip jq gettext \
    libsystemd-dev jq && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    pip install --no-cache-dir \
    yamllint==${YAMLLINT_VERSION} \
    yamlfixer-opt-nc==${YAMLFIXER_VERSION}

COPY --from=wolfi /usr/bin/clusterctl /usr/bin/clusterctl
COPY --from=wolfi /usr/bin/controller-gen /usr/bin/controller-gen
COPY --from=wolfi /usr/bin/helm /usr/bin/helm
COPY --from=wolfi /usr/bin/kubectl /usr/bin/kubectl
COPY --from=wolfi /usr/bin/kind /usr/bin/kind
COPY --from=wolfi /usr/bin/kustomize /usr/bin/kustomize
COPY --from=wolfi /usr/bin/skopeo /usr/bin/skopeo
COPY --from=wolfi /usr/bin/trivy /usr/bin/trivy
COPY --from=lychee /usr/bin/lychee /usr/bin/lychee
COPY --from=golangci /bin/golangci-lint /usr/local/bin
COPY --from=hadolint /bin/hadolint /usr/bin/hadolint

ENV GOCACHE=/go/cache

COPY build.sh /
RUN chmod +x /build.sh
ENTRYPOINT ["/build.sh"]
