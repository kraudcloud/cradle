cradle
======

the kraudcloud microvm

runs docker containers inside qemu on k8s.


## usage



```
kubectl apply -f example.yaml
```


now you can interact with it via the docker cli

```
kubectl exec -ti demo -- docker ps
```


