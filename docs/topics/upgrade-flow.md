# Upgrade flow

This flow assumes that you have an existing cluster that references a certain `ClusterClass` called `docker-ferrol-1-27-v1`.

There are two forms of updates: "normal" cluster stack updates, where you would update the above `ClusterClass` to `docker-ferrol-1-27-v2`, and updates of the Kubernetes minor version, e.g. `docker-ferrol-1-28-v1`. 

In both cases, you need to make sure that you have the respective `ClusterClass` available. This works a bit different in the two cases, as you will need a new `ClusterStack` object in the latter case that works with Kubernetes minor version 1.28. This is one of the properties of a `ClusterStack` that you specify in the spec.

After you made sure that you have the `ClusterClass` ready to which you want to upgrade, you can edit your `Cluster` object according to the following pattern:

Update `spec.topology.class` to the name of the new `ClusterClass` and change `spec.topology.version` to the respective Kubernetes version. This can be, for example, "1.28.1". You have to find out the right Kubernetes version for the respective cluster stack. 

You can either do this by checking the status of the `ClusterStack` object, or by fetching the `ClusterStackRelease` objects. You will find a `ClusterStackRelease` object that has the same name as your desired `ClusterClass`. This object has a property `status.kubernetesVersion` that shows you the version that you need to specify. 

Another option is to check the documentation of the cluster stack to find information about the respective releases.

Please note that `spec.topology.version` does not have to be specified if a mutating webhook fills the property automatically for you. Check out the latest release notes of the operator to verify whether that is implemented already.

