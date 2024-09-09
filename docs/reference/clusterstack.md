## ClusterStack

The `ClusterStack` object is the main resource for users to work with. It contains the most important details of a cluster stack and its releases (i.e. certain versions). In its status is the main source of information of the state of everything related to cluster stacks.



### Lifecycle of a ClusterStack

The `ClusterStack` object has a sub-resource `ClusterStackRelease` for every release that should be provided to the user, either by specifying them manually in the versions array, or automatically through the auto-subscribe functionality.

The controller reconciles the two sources of information and checks whether for every release that should exist, there is actually one. It also deletes `ClusterStackRelease` objects that are not required anymore.

Additionally, it fetches information from the `ClusterStackRelease` objects and populates its own state with it.

In case that a provider integration is used, it will create `ProviderClusterStackRelease` objects in addition to `ClusterStackRelease` objects, based on the `ProviderClusterStackReleaseTemplate` objects given as reference in `spec.providerRef`.


### Overview of ClusterStack.Spec

| Key                      | Type      | Default | Required | Description                                                                                                                                                                                                                                                                            |
| ------------------------ | --------- | ------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| provider                 | string       |         | yes      | Name of the provider, e.g. "docker". It is used in various places, e.g. while fetching the respective release assets or while naming resources (ClusterStackReleases, ProviderClusterStackResources, etc.).|
| name          | string    |         | yes       | Name of the cluster stack. It is used as well for fetching release assets and other tasks. |
| kubernetesVersion      | string    |         | yes      | Kubernetes version in the format `<majorVersion>.<minorVersion>`, e.g. 1.26. Specifies the Kubernetes minor version of the cluster stack that should be taken.|
| channel     | string    | stable         | no       | Name of release channel that is used, e.g. stable channel ("v1", "v2", etc.) or beta channel (e.g. "v0-beta.1").|
| versions | []string |         | no       | List of versions that the controller should make available of a cluster stack. Used only in case very specific versions are supposed to be used. Not required if always the latest versions should be made available. |
| autoSubscribe              | bool    | true        | no       | Specifies whether the controller should automatically check whether there are new releases of the cluster stack and if so automatically download them. |
| noProvider          | bool      |    false     | no       | If set to true, the controller does not expect any provider-specific objects and just focuses on applying Cluster API objects in management cluster and cluster addons in all workload clusters. |
| providerRef              | object    |         | no       | ProviderRef has to be specified if spec.noProvider is false. It references the ProviderClusterStackReleaseTemplate that contains all information to create the ProviderClusterStackRelease objects. |

### Example of the ClusterStack object

You should create one of these objects for each of your bare metal servers that you want to use for your deployment.

```yaml
apiVersion: clusterstack.x-k8s.io/v1alpha1
kind: ClusterStack
metadata:
  name: clusterstack
  namespace: cluster
spec:
  provider: docker
  name: ferrol
  kubernetesVersion: "1.27"
  channel: stable
  autoSubscribe: true
  noProvider: true
```
