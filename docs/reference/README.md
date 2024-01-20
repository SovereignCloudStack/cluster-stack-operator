# Object reference

In this reference, you will find all resources that are defined in this API. Please note that depending on the provider, there are also provider-specific custom resources that are important to use this operator. 

Have a look in the documentation of the respective provider integration to find more details of how the provider-specific custom resources look like and how they can be used. 

The only user-facing CRD of the Cluster Stack Operator is the [ClusterStack](clusterstack.md). Apart from that, there will be, depending on a provider, a `ProviderClusterStackReleaseTemplate`, which gives provider-specific information and has to be applied by the user.

To see all CRDs, including the ones that are not intended to be applied by users, see [here](https://doc.crds.dev/github.com/SovereignCloudStack/cluster-stack-operator).