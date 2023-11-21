# Deep dive: User flow

It is essential to understand the flow of what you have to do as a user and what happens in the background. 

The [Quickstart guide](quickstart.md) goes over all small steps you have to do to. If you are just interested in getting started, then have a look there.

In the following, we will not go into the detail of every command, but will focus more on a high-level of what you have to do and of what happens in the background. 

## Steps to create a workload cluster

### Get the right cluster stacks

The first step would be to make sure that you have the cluster stacks implemented that you want to use. Usually, you will use cluster stacks that have been implemented by others for the provider that you want to use. However, you can also build your own cluster stacks.

### Apply cluster stack resource

If you have everything available, you can start your management cluster / bootstrap cluster. In this cluster, you have to apply the `ClusterStack` custom resource with your individual desired configuration. 

Depending on your configuration, you will have to wait until all steps are done in the background.

The operator will perform all necessary steps to provide you with node images. If all node images are ready, it will apply the Cluster API resources that are required. 

At the end, you will have node images and Cluster API objects ready to use. There is only one step more to create a cluster.

### Use the ClusterClasses

 That the previous step is done, you can see in the status of the `ClusterStack` object. However, you can also just check if you have certain `ClusterClass` objects. The `ClusterClass` objects will be applied by the Cluster Stack Operator as well. They follow a certain naming pattern. If you have the cluster stack "ferrol" for the docker provider and Kubernetes version 1.27 in version "v1", then you'll see a `ClusterClass` that has the name "docker-ferrol-1-27-v1". 

 You can use this `ClusterClass` by referencing it in a `Cluster` object. For details, you can check out the official Cluster-API documentation.

### Wait until cluster addons are ready

If you created a workload cluster by applying a `Cluster` object, the cluster addons will be applied automatically. You just have to wait until everything is ready, e.g. that the CCM or CNI are installed. 

## Recap - how do Cluster API and Cluster Stacks work together?

The user triggers the flow by configuring and applying a `ClusterStack` custom resource. This will trigger some work in the background, to make node images and Cluster API objects ready to use.

This process is completed, when a `ClusterClass` with a certain name is created. This `ClusterClass` resource is used in order to create as many clusters as you want that look like the template specified in the `ClusterClass`. 

Upgrades of clusters are done by changing the reference to a new `ClusterClass`, e.g. from `docker-ferrol-1-27-v1` to `docker-ferrol-1-27-v2`. 

To sum up: The Cluster Stack Operator takes care of steps that you would otherwise have to do manually. It does not change anything in the normal Cluster API flow, expcept that it enforces the use of `ClusterClasses`. 

