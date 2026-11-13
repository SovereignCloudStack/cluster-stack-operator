# Test releases

The integration tests in `internal/test/integration/workloadcluster` use the `docker-scs-1-35-*` releases instead of
downloading them. They are built from the `docker/scs` cluster stack of
[SovereignCloudStack/cluster-stacks](https://github.com/SovereignCloudStack/cluster-stacks) (Kubernetes 1.35) and are
not edited by hand.

The `docker-ferrol-1-27-*` releases are in the old cluster stack format (sources in `test/cluster-stacks`). Only the
unit tests in `internal/controller` still use them, together with mocked clients.

| Release | Content |
|---|---|
| `docker-scs-1-35-v0-ttl-1792740950` | `docker/scs` 1-35 unchanged: addons `gateway-api`, `cni` and `metrics-server` |
| `docker-scs-1-35-v0-ttl-1792740963` | like the first release, but the `metrics-server` runs with two replicas and the `gateway-api` addon is removed |

The second release has the same Kubernetes version as the first one, so switching the cluster class takes the upgrade
path in which the objects of the new release are applied and the objects of the old release that are not part of the new
one are removed.

The `cni` and `metrics-server` charts are downloaded from their Helm repositories when the tests run, the releases only
contain the umbrella charts.

## Rebuilding

Build the releases with `hack/build.sh` in a checkout of cluster-stacks (`helm` and `yq` are required):

```shell
# first release: the stack as it is
OUTPUT_DIR=/tmp/releases ./hack/build.sh providers/docker/scs --version 1.35

# second release: a copy of the stack with a changed and a removed addon
mkdir -p /tmp/stack/docker/scs && cp -r providers/docker/scs/1-35 /tmp/stack/docker/scs/
rm -r /tmp/stack/docker/scs/1-35/cluster-addon/gateway-api
# remove the gateway-api stages from /tmp/stack/docker/scs/1-35/clusteraddon.yaml
# add "replicas: 2" below "fullnameOverride" in /tmp/stack/docker/scs/1-35/cluster-addon/metrics-server/values.yaml
OUTPUT_DIR=/tmp/releases ./hack/build.sh /tmp/stack/docker/scs --version 1.35
```

Copy the two directories from `/tmp/releases` to `test/releases/cluster-stacks/` and adjust the names in
`internal/test/integration/workloadcluster/controller_suite_test.go` (the release name has the dash notation, the cluster
class the dot notation of the version, e.g. `v0-ttl-1792740950` and `v0-ttl.1792740950`).
