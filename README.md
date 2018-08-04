# External-Loadbalancer

## Deploy 
*note*: you must have a kubernetes/openshift cluster up

On a kubernetes cluster run:
```
kubectl apply -f config/release/external-lb.yaml
```

On a openshift cluster run:

*note* you need to be logged as system:admin
``` 
oc apply -f config/release/external-lb.yaml
```

## Demo loadbalancer server

### Deploy
```
oc/kubectl apply -f config/samples/manager_v1alpha1_provider.yaml
```