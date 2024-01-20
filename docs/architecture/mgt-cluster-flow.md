# Management Cluster flow

In a Cluster API management cluster, the Cluster API operators run. In our management cluster, there are also the Cluster Stack operators. 

The user controls workload clusters via custom resources. As the Cluster Stack approach uses `ClusterClasses`, the user has to create only a `Cluster` object and refer to a `ClusterClass`.

However, in order for this to work, the `ClusterClass` has to be applied as well as all other Cluster API objects that are referenced by the `ClusterClass`, such as `MachineTemplates`, etc.

These Cluster API objects are packaged in a Helm Chart that is part of every cluster stack. The clusterstackrelease-controller is responsible for applying this Helm chart, which is done by first calling `helm template` and then the "apply" method of the Kubernetes go-client. 

The main resource is always the `ClusterClass` that follows a very specific naming pattern and is called in the exact same way as the `ClusterStackRelease` object that manages it. For example, `docker-ferrol-1-27-v1`, which refers to all defining properties of a specific release of a cluster stack for a certain provider.