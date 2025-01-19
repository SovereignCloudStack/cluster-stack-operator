# Getting Started

## Quickstart

Currently, there is a [demo](https://github.com/SovereignCloudStack/cluster-stacks-demo) that can be used to see how the Cluster Stack approach can work. It uses the Docker Provider Integration for Cluster API.

## Installation

### Prerequisites

```
- clusterctl
- envsubst
- kubectl
- Helm (optional)
```

#### Cluster API

We assume access to a Kubernetes cluster. To start
[kind](https://kind.sigs.k8s.io/) can be used for that.
This Kubernetes cluster must act as Cluster API management cluster.
If not already done, to make your Kubernetes cluster a management cluster by
installing the Cluster API objects, you can do

```
clusterctl init
```

### Configure Release Source

The Cluster Stack Operator downloads Cluster Stack releases either from GitHub
Releases or from an OCI registry.
Depending on which method is preferred, environment variables must be set to
give the Cluster Stack Operator access to the specific source.

#### Using GitHub Releases

> Be aware that GitHub enforces limitations on the number of API requests per
> unit of time. To overcome this, it is recommended to configure a [personal
> access token](https://github.com/settings/personal-access-tokens/new) for
> authenticated calls. This will significantly increase the rate limit for GitHub
> API requests. Fine grained PAT with `Public Repositories (read-only)` is
> enough.

Following variables tells the Cluster Stack Operator to look for releases in
`https://github.com/SovereignCloudStack/cluster-stacks`

```bash
export GIT_PROVIDER_B64=Z2l0aHVi  # github
export GIT_ORG_NAME_B64=U292ZXJlaWduQ2xvdWRTdGFjaw== # SovereignCloudStack
export GIT_REPOSITORY_NAME_B64=Y2x1c3Rlci1zdGFja3M=  # cluster-stacks
export GIT_ACCESS_TOKEN_B64=$(echo -n '<my-personal-access-token>' | base64 -w0)
```

#### Using OCI Registry

Following variables tells the Cluster Stack Operator to look for releases in
`https://registry.scs.community/kaas/cluster-stacks`

```bash
export OCI_REGISTRY_B64=cmVnaXN0cnkuc2NzLmNvbW11bml0eQ== # registry.scs.community
export OCI_REPOSITORY_B64=cmVnaXN0cnkuc2NzLmNvbW11bml0eS9rYWFzL2NsdXN0ZXItc3RhY2tzCg== # registry.scs.community/kaas/cluster-stacks
```

If the registry is not public the Cluster Stack Operator please also provide
`OCI_USERNAME_B64` and `OCI_PASSWORD_B64` or
`OCI_ACCESS_TOKEN_B64`

### Install Cluster Stack Operator

#### Install by manifest

```bash
# Get the latest CSO release version and apply CSO manifests
curl -sSL https://github.com/SovereignCloudStack/cluster-stack-operator/releases/latest/download/cso-infrastructure-components.yaml | envsubst | kubectl apply -f -
```

##### Enable OCI registry as source

Since GitHub is set as default source for the Cluster Stack releases, this can be changed with a patch:

```bash
kubectl patch deployment -n cso-system cso-controller-manager --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/command", "value": ["/manager", "-source", "oci"]}]'
```

#### Install with Helm (experimental)

There are simple Helm charts available which were created especially to make the switch between the sources easier.

```bash
helm upgrade -i cso -n cso-system \
    --create-namespace oci://registry.scs.community/cluster-stacks/cso \
    --set controllerManager.manager.source=oci \
    --set clusterStackVariables.ociRepository=registry.scs.community/kaas/cluster-stacks
```
