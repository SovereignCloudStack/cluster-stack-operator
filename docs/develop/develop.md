# Contributing

## Develop Cluster Stack Operator


Developing our operator is quite easy. First, you need to install some base requirements: Docker and Go. Second, you need to configure your environment variables. Then you can start developing with the local Kind cluster and the Tilt UI to create a workload cluster that is already pre-configured.

## Setting Tilt up
1. Install Docker and Go. We expect you to run on a Linux OS.
2. Create an ```.envrc``` file and specify the values you need. See the .envrc.sample for details.

## Developing with Tilt

<p align="center">
<img alt="tilt" src="./docs/pics/tilt.png" width="800px" />
</p>

Operator development requires a lot of iteration, and the “build, tag, push, update deployment” workflow can be very tedious. Tilt makes this process much simpler by watching for updates and automatically building and deploying them. To build a kind cluster and to start Tilt, run:

```shell
make tilt-up
```
> To access the Tilt UI please go to: `http://localhost:10351`


You should make sure that everything in the UI looks green. If not, e.g. if the clusterstack has not been synced, you can trigger the Tilt workflow again. In case of the clusterstack button this might be necessary, as it cannot be applied right after startup of the cluster and fails. Tilt unfortunately does not include a waiting period.

If everything is green, then you can already check for your clusterstack that has been deployed. You can use a tool like k9s to have a look at the management cluster and its custom resources.

Example:

```shell
❯ kubectl get clusterstacks -A
NAMESPACE   NAME           PROVIDER   CLUSTERSTACK   K8S    CHANNEL   AUTOSUBSCRIBE   USABLE   LATEST                            AGE     REASON   MESSAGE
cluster     clusterstack   docker     ferrol         1.27   stable    false           v2       docker-ferrol-1-27-v2 | v1.27.3   4m52s
```

```shell
❯ kubectl get clusterstackreleases.clusterstack.x-k8s.io -A
NAMESPACE   NAME                    K8S VERSION   READY   AGE     REASON   MESSAGE
cluster     docker-ferrol-1-27-v2   v1.27.3       true    7m51s
```

The above cluster stack was downloaded from [SovereignCloudStack/cluster-stacks](https://github.com/SovereignCloudStack/cluster-stacks/releases)

In case your clusterstack shows that it is ready, you can deploy a workload cluster. This could be done through the Tilt UI, by pressing the button in the top right corner "Create Workload Cluster". This triggers the `make create-workload-cluster-docker`, which uses the environment variables and the cluster-template.

In case you want to change some code, you can do so and see that Tilt triggers on save. It will update the container of the operator automatically.

If you want to change something in your ClusterStack or Cluster custom resources, you can have a look at `.cluster.yaml` and `.clusterstack.yaml`, which Tilt uses.

To tear down the workload cluster press the "Delete Workload Cluster" button. After a few minutes the resources should be deleted.

To tear down the kind cluster, use:

```shell
$ make delete-bootstrap-cluster
```

If you have any trouble finding the right command, then you can use `make help` to get a list of all available make targets.
