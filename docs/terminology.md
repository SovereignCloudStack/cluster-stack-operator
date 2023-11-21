# Terminology

The Cluster Stacks are a framework and provide tools how to use them. The terminology is not perfect and if you have an idea how to improve it, please reach out. Right now there are the following terms:

## Cluster Stack framework

The framework of cluster stacks refers to the fact that cluster stacks of any shape can be created for any provider that supports Cluster API. The framework has no opinion about how the Cluster Stacks have to look like, which configuration of node images and Kubernetes you use and which cluster addons you include. 

As long as a cluster stack is implemented and released in a correct way, the cluster-stack-operator will be able to use it, fully independent of the detailed architectural decisions that were taken with regards to how the clusters that come out of this cluster stack should look like. 

This flexibility is meant by the term "framework".

## A definition of a cluster stack

A definition of a cluster stack is a document describing, independent of any provider, how a cluster stack xyz should look like. On a very high level, this could be something like "we want to use Ubuntu node images, basic Kubeadm and Cilium as CNI".

There is no template for such a definition and no pre-defined structure how such a definition should look like. 

A definition of a cluster stack xyz can be used as a base to implement this cluster stack xyz for providers a and b.

## An implementation of a cluster Stack

A cluster stack can be implemented for a certain provider. The collection of configuration code, Helm charts, etc. is what we call an implementation of a cluster stack. The release assets that have to be generated from that is what people actually use, usually with the Cluster Stack Operator.

## Cluster Stack Operator

A Kubernetes operator that works with the release assets and applies resources in management and each workload cluster. It works together with Provider Integrations, if they are needed.

## Cluster Stack Provider Integration

Provider integrations are needed if a user has to do manual steps on a provider to use custom node images for the nodes in a cluster. If no such steps are required, then the provider integration is not needed.

## ClusterStack as custom resource definition

The `ClusterStack` is a CRD that the user interacts with directly. It shows in its status the state of the respective versions of this cluster stack that the user wants to have. 