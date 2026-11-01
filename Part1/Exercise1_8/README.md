## Accessing the application

The application is exposed through an Ingress, with a ClusterIP Service as its backend rather than a NodePort. Apply the Kubernetes resources from this directory:

```sh
kubectl apply -f manifests/
```

Once the Ingress controller is running, access the application at its configured Ingress address. Path-based routing alongside the Log output application will be configured in a later exercise.
