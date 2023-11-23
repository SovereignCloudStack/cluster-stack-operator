# Write a Provider for Cluster Stack Operator

This document describes how to create your own (infrastructure) provider for the Cluster Stack Operator. Before you start, please familiarize yourself with [Cluster Stacks](https://github.com/SovereignCloudStack/cluster-stacks/blob/main/README.md) and [Cluster Stack Operator](https://github.com/SovereignCloudStack/cluster-stack-operator/blob/main/README.md).

## Key Concepts

- Cluster API: Cluster API is a Kubernetes sub-project focused on providing declarative APIs and tooling to simplify provisioning, upgrading, and operating multiple Kubernetes clusters. Read more about Cluster API [here](https://cluster-api.sigs.k8s.io/).

- Cluster Stack: Cluster Stack is a framework built on top of the Cluster API and ClusterClasses. It defines a way of defining templates of clusters that contain all necessary components and configurations to create potentially production-ready clusters. Implementations of a cluster stack are released and can be used by following a certain order of steps.

 There are three components of a cluster stack:
    1. Cluster addons: The cluster addons (CNI, CSI, CCM) have to be applied in each workload cluster that the user starts.
    2. Cluster API objects: The ClusterClass object makes it easier to use Cluster-API. The cluster stack contains a ClusterClass object and other Cluster-API objects that are necessary in order to use the ClusterClass. These objects have to be applied in the management cluster.
    3. Node images: Node images can be provided to the user in different form. They are released and tested together with the other two components of the cluster stack.

- Cluster Stack Operator: Cluster Stack Operator is a Kubernetes Operator that automates all the manual steps required to manage a Cluster Stack.

## Architecture

![Architecture](https://github.com/SovereignCloudStack/cluster-stacks-demo/blob/main/hack/images/syself-cluster-stacks-web.png?raw=true)

This diagram shows the overall layout of how Cluster Stacks sits in the whole ecosystem. We're going to focus on the Cluster Stack Operator and the provider integration.

## Why do we need a provider integration?

The first two components, cluster addons and Cluster-API objects, are provided by Helm charts that can be applied in the same way on all providers. The node images, however, are heavily provider-specific. Therefore, some code that provides node images for users, is required. 

## When do we NOT need a provider integration?

If a provider has no node images (e.g. cluster-api-provider-docker), or does not require users to do manual steps to access these node images, then no provider integration is needed. In this case, the ClusterStack custom resource can be configured in `noProvider` mode. 



The purpose of a provider integration is to ensure node images can be built, and shared in a provider specific way, and that the Cluster Stack Operator can use these images to create workload clusters.


### How does this work?

As the Cluster Stack Operator is closely related to Cluster API, many patterns can be copied. One of them is the structure of a core operator as well as provider integrations, that are in turn operators. 

For each Cluster API Provider Integration, there might be a Cluster Stack Provider Integration as well. See [here](#when-do-we-not-need-a-provider-integration) to decide whether a provider integration is needed or not. 

The approach of building a separate operator brings the freedom to decide how the custom resources should look like and what the reconcile loop should do.

There are some relations between the core operator as well as the provider integration. The core operator reads from and expects two custom resources:
`ProviderClusterStackReleaseTemplate` and `ProviderClusterStackRelease`. The template object provides a template, similar to `MachineTemplate` of Cluster API and has to follow the same structure of the Cluster API templates. The `ProviderClusterStackRelease` needs to have the property `status.ready`, a boolean, which is read by the clusterstackrelease-controller to find out whether the provider-specific work is completed or not.  

## What does the Cluster Stack Operator do with provider-specific objects?

The ClusterStack CRD has a `spec.provider` field as well as `spec.noProvider`. If the `spec.noProvider` boolean is set to `true`, then no provider-specific tasks are done for the cluster stack. Otherwise, Cluster Stack Operator looks for the `ProviderClusterStackReleaseTemplate` object that is referenced in the `spec.provider` field. Based on this template, the clusterstack-operator will create one `ProviderClusterStackRelease` object for each `ClusterStackRelease` object. These two objects have a one-on-one relationship. 

The clusterstackrelease-controller will search for the `ProviderClusterStackRelease` object that is referenced in the spec of `ClusterStackRelease` and check whether `status.ready` is `true`. If it is not, it will wait. Only if the provider-specific jobs are completed and the `status.ready` boolean is set on `true`, the reconcile loop will proceed and apply the required Cluster API objects in the management cluster.

It makes sense to wait, because otherwise the `ClusterClass` object would be present that allows the user to use the respective release of the cluster stack. However, if the node images are not ready yet, the cluster cannot start successfully. Therefore, to create the `ClusterClass` object is the last step to be done.

## Provider Contract

A Cluster Stack Provider Integration ensures that the node images that are released e.g. as build information in a release of a cluster stack, are accessible and ready to use. A user who creates a workload cluster via the the `ClusterClass` object of a release of a cluster stack should have all node images provided.


A Cluster Stack Provider should be built as a separate Kubernetes Operator.

### Custom resources

#### ProviderClusterStackRelease resource

The `ProviderClusterStackRelease` resource must have a `status` field `ready` (boolean), indicating that the provider-specific tasks (notably providing node images) are completed for the respective release.

#### ProviderClusterStackReleaseTemplate resource

For a `ProviderClusterStackRelease` resource, there must be a corresponding template resource following this pattern:

```
// ProviderClusterStackReleaseTemplateSpec defines the desired state of ProviderClusterStackReleaseTemplate.
type ProviderClusterStackReleaseTemplateSpec struct {
	Template ProviderClusterStackReleaseTemplateResource `json:"template"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=providerclusterstackreleasetemplates,scope=Namespaced,categories=cluster-stack,shortName=pcrt
// +kubebuilder:storageversion

// ProviderClusterStackReleaseTemplate is the Schema for the providerclusterstackreleasetemplates API.
type ProviderClusterStackReleaseTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderClusterStackReleaseTemplateSpec `json:"spec,omitempty"`
}

type ProviderClusterStackReleaseTemplateResource struct {
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`
	Spec ProviderClusterStackReleaseSpec `json:"spec"`
}
```

## Implementing a new Provider Integration

// TODO This has to be refined based on https://cluster-api.sigs.k8s.io/developer/providers/implementers-guide/overview.
// The goal is that users can set up new repositories by themselves and know everything that is needed.

## Some ideas to implement good provider integrations

In case that there are multiple node images which have to be provided to the user, it makes sense to have separate custom resources so that one custom resource is responsible for only one node image. A pattern where there is a `ProviderClusterStackRelease` that creates different `ProviderNodeImageRelease` custom resources, one for each node image that should be built, is advisable. 

The same pattern that is also used in the objects of `ClusterStackRelease` and `ProviderClusterStackRelease` can be also applied here in a way that `ProviderClusterStackRelease` is set on `ready: true` if and only if all `ProviderNodeImageRelease` custom resources are also `ready: true`.

This pattern also easily allows node images to be re-used. For example, if one cluster stack has the same node images in one version and the next, then they can be re-used and don't have to be necessarily re-created. This pattern can be easily implemented here by setting additional owner references of `ProviderClusterStackRelease` objects to `ProviderNodeImageRelease` objects, if they exist already.

