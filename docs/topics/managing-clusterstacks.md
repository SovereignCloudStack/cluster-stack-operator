# Managing ClusterStack objects

The `ClusterStack` object is the central resource that you have to work with. You have to specify a provider, the name of the cluster stack you want to use, as well as the Kubernetes minor version. 

If you want to use multiple different Kubernetes minor versions, you will have to create multiple `ClusterStack` objects. The same goes for multiple providers, or multiple cluster stacks (e.g. ferrol) that might have different features.

In order to use a cluster stack in a specific version, you have two options: first, you can specify a list of versions in `spec.versions`. Second, you can enable `autoSubscribe`, so that the operator will automatically check for the latest version and make it available to you. 

Usually, you will always want to use auto-subscribe, so that the operator takes care of providing you with the latest versions.
