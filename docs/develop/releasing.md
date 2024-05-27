## Release Process
This document describes the release process of cluster-stack-operator. 
The release process is done using GitHub actions. The release workflow triggers when a tag is pushed starting with `v*` 

> [!NOTE]
> Before the release, please make sure that you've already updated metadata.yaml at the root of the repo. So, if you're using v1.2.0 then you'll also need to update the `releaseSeries` block in metadata.yaml. The tag used for releasing should be compatible with what we have defined in metadata.yaml

Creating a new release of cluster-stack-operator covers the following steps: 

### Step 1: Create and Push a tag 
1. Create an annotated tag 
```bash
git switch main
git pull --rebase 
# check older releases for semver compatibility
export RELEASE_TAG=<the tag of the release to be cut> (e.g. v0.0.1)
git tag -a ${RELEASE_TAG} -m ${RELEASE_TAG}
```
2. Push the tag to GitHub repository 

> [!NOTE]  
> `origin` should be the name of the remote pointing to https://github.com/SovereignCloudStack/cluster-stack-operator and you should have permission to push tags to the repository.

Once you confirm that origin is correct then push the tag by invoking the following command:
```bash
git push origin ${RELEASE_TAG}
```
This will automatically trigger a Github workflow to create a draft release in GitHub.

### Step 2: Release in GitHub
1. Review the draft release manually and check if the image tags are correct in the released manifest.
2. After this, if you're going to cut a pre-release version then please append this line to the top of the release notes. 
> [!WARNING]
> :rotating_light: This is a RELEASE CANDIDATE. If you find any bugs, please file an [issue](https://github.com/SovereignCloudStack/cluster-stack-operator/issues/new).
3. Before publishing the images make sure that images are already there in GitHub container registry. This can be checked in the [packages section](https://github.com/SovereignCloudStack/cluster-stack-operator/pkgs/container/cso) of the organisation.
4. Publish the release.
