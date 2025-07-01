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
FROM docker.io/library/alpine:3.22.0 AS lychee
# update: datasource=github-tags depName=lycheeverse/lychee versioning=semver
ENV LYCHEE_VERSION="v0.15.1"
# hadolint ignore=DL3018
# update: datasource=github-tags depName=lycheeverse/lychee versioning=semver
ENV LYCHEE_VERSION="v0.15.1"
RUN apk add --no-cache curl && \
    curl -L -o /tmp/lychee-${LYCHEE_VERSION}.tgz https://github.com/lycheeverse/lychee/releases/download/${LYCHEE_VERSION}/lychee-${LYCHEE_VERSION}-x86_64-unknown-linux-gnu.tar.gz && \
    tar -xz -C /tmp -f /tmp/lychee-${LYCHEE_VERSION}.tgz && \
    mv /tmp/lychee /usr/bin/lychee && \
    rm -rf /tmp/linux-amd64 /tmp/lychee-${LYCHEE_VERSION}.tgz

FROM cgr.dev/chainguard/wolfi-base:latest AS wolfi

# update: datasource=github-tags depName=kubernetes-sigs/cluster-api
ARG CLUSTERCTL_VERSION="v1.8.1"
# update: datasource=github-tags depName=helm/helm
ENV HELM_VERSION="v3.18.3"
# update: datasource=github-tags depName=kubernetes-sigs/kind
ARG KIND_VERSION="v0.24.0"
# update: datasource=github-tags depName=kubernetes/kubernetes
ARG KUBECTL_VERSION="v1.30.4"
# update: datasource=github-tags depName=kubernetes-sigs/kustomize extractVersion=^kustomize\/v(?<version>.+)$
ARG KUSTOMIZE_VERSION="v5.4.3"
# update: datasource=github-tags depName=aquasecurity/trivy
ARG TRIVY_VERSION="v0.54.1"
# update: datasource=github-tags depName=kubernetes-sigs/controller-tools
ARG CONTROLLER_GEN_VERSION="v0.16.1"

# hadolint ignore=DL3018
RUN apk add -U --no-cache \
    curl \
    clusterctl=~${CLUSTERCTL_VERSION#v} \
    controller-gen=~${CONTROLLER_GEN_VERSION#v} \
    helm=~${HELM_VERSION#v} \
    kind=~${KIND_VERSION#v} \
    kubectl=~${KUBECTL_VERSION#v} \
    kustomize=~${KUSTOMIZE_VERSION#v} \
    trivy=~${TRIVY_VERSION#v}

# Install Golang CI Lint
FROM docker.io/library/alpine:3.22.0 AS golangci
# update: datasource=github-tags depName=golangci/golangci-lint versioning=semver
ENV GOLANGCI_VERSION="v2.2.1"
WORKDIR /
# hadolint ignore=DL3018,DL4006
RUN apk add --no-cache curl && \
    curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s ${GOLANGCI_VERSION}

# Install Hadolint
FROM docker.io/hadolint/hadolint:v2.12.0-alpine@sha256:3c206a451cec6d486367e758645269fd7d696c5ccb6ff59d8b03b0e45268a199 AS hadolint

# Install Trivy
FROM docker.io/aquasec/trivy:0.64.0 AS trivy

############################
# CSO Build Image Base #
############################
FROM docker.io/library/golang:1.23.5-bullseye

# update: datasource=github-tags depName=adrienverge/yamllint versioning=semver
ENV YAMLLINT_VERSION="v1.37.1"
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
COPY --from=wolfi /usr/bin/trivy /usr/bin/trivy
COPY --from=lychee /usr/bin/lychee /usr/bin/lychee
COPY --from=golangci /bin/golangci-lint /usr/local/bin
COPY --from=hadolint /bin/hadolint /usr/bin/hadolint

ENV GOCACHE=/go/cache

COPY build.sh /
RUN chmod +x /build.sh
ENTRYPOINT ["/build.sh"]
