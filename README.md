# Kubernetes Greeting Operator

This operator creates a dead simple greeting server in a Kubernetes environment.
The server always answers with his name when requesting `/greet`.

```
curl http://localhost:8080/greet
I am Foo Bar
```

# How to use

Build the operator and server docker images.

```
make images
```

Be sure they are available in the cluster and then apply the Kubernetes manifests which deploy the operator and the permissions required.

```
kubectl apply -f ./k8s
```
