# Cluster Stack Operator

## Get started

The Cluster Stack Operator works with [Cluster Stacks](https://github.com/SovereignCloudStack/cluster-stacks) and creates Kubernetes clusters in an easy and [Cluster-API](https://github.com/kubernetes-sigs/cluster-api)-native way.

The operator can be used with any repository that contains releases of cluster stacks. For more details of how to implement them, have a look at the [cluster-stacks repository](https://github.com/SovereignCloudStack/cluster-stacks).

To try out this operator and cluster stacks, have a look at this [demo](https://github.com/SovereignCloudStack/cluster-stacks-demo).

## Why Cluster Stacks?

Kubernetes and Cluster API enable self-service Kubernetes. But do they take care of everything? No! Both tools solve one specific purpose perfectly and leave other tasks out of scope. 

Therefore, a user has to answer questions like these: how do I get node images? How can I manage core cluster components (e.g. CCM, CNI)? How can I safely and efficiently upgrade Kubernetes clusters? 

The Cluster Stacks give an answer by working hand-in-hand with Cluster API to facilitate self-service Kubernetes. They provide a framework and tools for managing a fully open-source self-service Kubernetes infrastructure with ease. They integrate seamlessly in the Cluster API cosmos. 

The Cluster Stack operator enables an “Infrastructure as Software” approach for managing Kubernetes clusters in self-service.

The Cluster Stacks are very generic and can be adapted to many use cases. 

### Are Cluster Stacks relevant to you?

Are you interested in setting up Kubernetes in your company based on open-source software? Do you not want to rely on other providers but own your Kubernetes clusters? Do you want to manage Kubernetes clusters for others? Do you plan on using Cluster API?

In all of these cases, the Cluster Stacks are for you! 

They make it easy to build a self-service Kubernetes infrastructure for internal use, as well as a to create a Managed Kubernetes offering.


## Further documentation

Please have a look at our [docs](docs/README.md) to find more information about the architecture, how to get started, how to develop this operator or provider integrations, and much more.