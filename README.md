# Cluster Stack Operator

## Get started

The Cluster Stack Operator works with [Cluster Stacks](https://github.com/SovereignCloudStack/cluster-stacks) and creates Kubernetes clusters in an easy and [Cluster-API](https://github.com/kubernetes-sigs/cluster-api)-native way.

The operator can be used with any repository that contains releases of cluster stacks. For more details of how to implement them, have a look at the [cluster-stacks repository](https://github.com/SovereignCloudStack/cluster-stacks).

To try out this operator and cluster stacks, have a look at this [demo](https://github.com/SovereignCloudStack/cluster-stacks-demo).

## What is the Cluster Stack Operator?

The Cluster Stack Operator facilitates the manual work that needs to be done to use cluster stacks. 

There are three components of a cluster stack:

1. Cluster addons: The cluster addons (CNI, CSI, CCM) have to be applied in each workload cluster that the user starts
2. Cluster API objects: The `ClusterClass` object makes it easier to use Cluster-API. The cluster stack contains a `ClusterClass` object and other Cluster-API objects that are necessary in order to use the `ClusterClass`. These objects have to be applied in the management cluster.
3. Node images: Node images can be provided to the user in different form. They are released and tested together with the other two components of the cluster stack.

The first two are handled by this operator here. The node images, on the other hand, have to be handled by separate provider integrations, similar to the ones that [Cluster-API uses](https://cluster-api.sigs.k8s.io/developer/providers/implementers-guide/overview).

## Implementing a provider integration

Further information and documentation on how to implement a provider integration will follow soon.