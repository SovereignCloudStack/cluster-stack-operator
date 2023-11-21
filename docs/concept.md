
# Understanding the concept of Cluster Stacks

The Cluster Stack framework was developed as one of the building blocks of an open-source Kubernetes-as-a-Service. The goal was to make it easier and more user-friendly to manage Kubernetes and Cluster API.

## Cluster Stacks and Cluster API

Do Cluster Stacks replace Cluster API? No! Do Cluster Stacks use Cluster API internally? No! The Cluster Stack framework accompanies Cluster API, but is on the same level of the hierarchy. The Cluster Stack approach does not wrap Cluster API into something else, but adds a few tools next to it. The Cluster Stacks are meant to take over some tasks that are relevant to the user, but for which Cluster API has no opinion. 

As a user of Cluster Stacks, you will see that you have an opinionated way of using Cluster API, for example by enforcing the use of ClusterClasses, but in the end, it is still vanilla Cluster API. However, instead of, for example, having to manage core cluster components, the so-called cluster addons, yourself, the cluster-stack-operator takes care of this.

By installing the required CRDs as well as the cluster-stack-operator next to the Cluster API CRDs and operators into the Cluster API management cluster, you can start using the Cluster Stacks!

To sum up: everything you know about Cluster API still applies when using the Cluster Stack Framework!

## Why cluster stacks?

Cluster stacks solve multiple issues users face when using Cluster API. Here a selection of them:

- Cluster API assumes that node images are available. This might mean some manual work for the user, which is completely out of scope for Cluster API.
- Cluster API does not have a stable solution to manage core cluster components that every workload cluster needs (cloud controller manager, container network interface, etc.). There is some work around so-called "add-on providers", but this is a very recent development.
- Upgrading clusters is challenging, as there might be incompatibilites between the various components (configurations, applications, etc.). Many users don't regularly upgrade clusters because of that.
- Cluster API has some downsides with regards to user experience, as there are many different objects that a user has to apply and manage.

The cluster stack approach tries to solve all of the above issues and tries to connect everything that users need in order to manage a fleet of Kubernetes clusters efficiently and easily. 

At the same time, it acknowledges the ease of Cluster API and uses it as its core component. Instead of re-inventing the wheel, Cluster API is extended with relevant and meaningful additions that improve the user experience.

## What do cluster stacks NOT try to do?

Cluster stacks concentrate on providing users with all necessary Cluster API objects in the management cluster, on providing node images (according to the demands of the respective provider), as well as core components of the workload clusters. The cluster stacks aim to provide a way of testing and versioning full templates of clusters.

However, they also aim to fulfill their purpose in a similar way of Cluster API by concentrating on one very important part and do that very well.

If there are any other use cases, e.g. installing applications automatically in workload clusters (an observability stack, GitOps tools, etc.), then this is a use case that is outside of the cluster stacks functionality. 

They are not intended to incorporate all features that users might want, but they can easily go hand-in-hand with other tools that enhance Cluster API.

## Integrating cluster stacks with other tools

The Cluster API cosmos is large and there are many tools around that can prove useful. Cluster stacks should be compatible with most of the other tools that you might want to use or build, as long as they follow the same pattern of using the declaritive approach of having custom resources that are reconciled by operators.

The cluster stacks can be used via custom resources and an operator reconciling them. Custom resources allow users to extend the Kubernetes API according to their needs. 

If you want to add your own functionality, you can also define your CRDs and write operators to reconcile them. If you think that your idea is very generic and might be interesting for the community in general, then reach out to the SCS team. Together, we will be able to improve the user experience even further!
