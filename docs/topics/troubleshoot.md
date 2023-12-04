
## Troubleshooting

Check the latest events:

```shell
kubectl get events -A  --sort-by=.lastTimestamp
```

Check the conditions:

```shell
go run github.com/guettli/check-conditions@latest all 
```

Check with `clusterctl`:

```shell
clusterctl describe cluster -n cluster my-cluster
```

Check the logs. List all logs from all deployments. Show the logs of the last ten minutes:

```shell
kubectl get deployment -A --no-headers | while read -r ns d _; do echo; echo "====== $ns $d"; kubectl logs --since=10m -n $ns deployment/$d; done
```
