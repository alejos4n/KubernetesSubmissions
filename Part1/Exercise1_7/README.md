## Assignment

> 
> "Log output" application currently outputs a timestamp and a random string to the logs.
> 
> Add an endpoint to request the current status (timestamp and string) and an ingress so that you can access it with a browser.
> 
> You can just store the string and timestamp to the memory.

The Go application and its Dockerfile are in `app/`; Kubernetes resources are in `manifests/`. From this exercise directory, build the image with:

```sh
docker build -f app/Dockerfile .
```

Apply the Kubernetes resources with:

```sh
kubectl apply -f manifests/
```
