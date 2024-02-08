This is a quickstart guide on using the local clusterstacks. 

Requirements: 
- docker
- kind 
- make

### Introduction 
Our controller interacts with cluster-stacks and they are stored inside GitHub releases when used in production. While this is the ideal combination to run our controller it's not a great experience when we want to develop or test new cluster-stacks.
To avoid this problem, we introduced `local_mode` and in this mode we start our controller by passing some flags and instead of using remote cluster-stacks, we put them inside a directory locally.
This way developers working on developing new cluster-stacks get a more better experience developing cluster-stacks and iterating fast on the same.

### using right flags 
Cluster stacks in local_mode do not fetch release assets directly from GitHub and to work with that we first need to download that and use the correct flags when we start the container.
The flags we are using in this repo is following:
```bash
  args:
    - --leader-elect=true
    - --release-dir=/tmp
    - --local=true
```
What this means is that, the controller will look for release assets under /tmp directory inside the contanier. If the controller doesn't find release asset under `/tmp` directory then you can get an error in the status of `clusterstackrelease` object.

### getting release assets 

There are several ways to get release assets. Two common ways:

* From a Github release
* From a directory created by [csmctl](https://github.com/SovereignCloudStack/csmctl)

Overall it does not matter who you received the directory.

Tilt expects the release assets under `.release` directory of the repository.

#### getting release assets: From Github

To download a release asset from Github use the following commands.

```bash
mkdir -p .release/docker-ferrol-1-27-v2
cd .release/docker-ferrol-1-27-v2
gh release download -R sovereigncloudstack/cluster-stacks docker-ferrol-1-27-v2
```
You can also fetch the release asset manually. Using `gh` makes it a little bit easier to download all the release asset just by invoking one command. 

Your local `.release` directory should have the following structure.
```bash
$ tree .release/
.release/
└── docker-ferrol-1-27-v2
    ├── cluster-addon-values.yaml
    ├── docker-ferrol-1-27-cluster-addon-v2.tgz
    ├── docker-ferrol-1-27-cluster-class-v2.tgz
    ├── metadata.yaml
    ├── node-images.yaml
    └── topology-docker.yaml

2 directories, 6 files
```

*NOTE: This directory structure is very important and you should have the same in order for the controller to work and sync.*

#### getting release assets: From csmctl

If you use [csmctl](https://github.com/SovereignCloudStack/csmctl), then it creates the required directory under `release`.

Example:

```
❯ csmctl create tests/cluster-stacks/docker/ferrol -m hash
Created releases/docker-ferrol-1-27-v0-sha-7ff9188
```

Copy this directory to .release:

```
cp -a releases/docker-ferrol-1-27-v0-sha-7ff9188 ../cluster-stack-operator/.release
```


### using local mode (toggling local mode)
From a user perspective who is using tilt to develop and test local cluster-stacks, we just have to make one change in Tiltfile and restart the complete setup all over again.

For making changes in tilt environment, we use `tilt-settings.yaml` file. If you don't have it then please copy it from `tilt-settings.yaml.example` which is at the root of the repo using the following command. 
```bash
cp tilt-settings.yaml.example tilt-settings.yaml
```
Now, since you have your `tilt-settings.yaml` ready, you have to edit `local_mode` and make it to `true`
Your `tilt-settings.yaml` file should look following:
```yaml
allowed_contexts:
  - kind-cso
local_mode: false
# truncated
```

Once you're toggled `local_mode` to `true`, you're ready to start your tilt setup. Use the following command to start it.  
```bash
make tilt-up
```
Once the setup is ready and you've the cluster-stack-operator pod running, you can exec into the container and check the `/tmp` directory for the presence of cluster-stacks. 

### Troubleshooting 

```bash
$ kubectl -n cso-system exec -it cso-controller-manager-5bdc445647-j4p9p -- sh
/ # tree /tmp/
/tmp/
└── cluster-stacks
    └── docker-ferrol-1-27-v2
        ├── cluster-addon-values.yaml
        ├── docker-ferrol-1-27-cluster-addon-v2.tgz
        ├── docker-ferrol-1-27-cluster-class-v2.tgz
        ├── metadata.yaml
        ├── node-images.yaml
        └── topology-docker.yaml

2 directories, 6 files
```
You can now click on `+` button on the right hand side of tilt UI to create a workload cluster. Once the cluster is created you can check the status of the components via the following commands. 
```bash
 $ kubectl get clusterstack -A
NAMESPACE   NAME           PROVIDER   CLUSTERSTACK   K8S    CHANNEL   AUTOSUBSCRIBE   USABLE   LATEST                            AGE     REASON   MESSAGE
cluster     clusterstack   docker     ferrol         1.27   stable    false           v2       docker-ferrol-1-27-v2 | v1.27.3   5m16s

 $ kubectl get clusterstackreleases.clusterstack.x-k8s.io -A
NAMESPACE   NAME                    K8S VERSION   READY   AGE     REASON   MESSAGE
cluster     docker-ferrol-1-27-v2   v1.27.3       true    5m16s

 $ kubectl get clusters -A
NAMESPACE   NAME          CLUSTERCLASS            PHASE         AGE     VERSION
cluster     test-dfkhje   docker-ferrol-1-27-v2   Provisioned   2m46s   v1.27.3
```
