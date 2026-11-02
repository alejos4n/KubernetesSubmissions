# Todo app with a persistent image cache

The app displays a random image from Lorem Picsum at `GET /`. It downloads an image from `https://picsum.photos/1200` when the cache is empty and stores the image plus its refresh state in `/data`. The cache is on a PersistentVolume, so it survives container and pod restarts.

An image is fresh for 10 minutes. The first request after it expires may receive the old image once; the next request fetches and caches a new one. If the image service is temporarily unavailable after expiry, the existing cached image is served and the refresh is retried on the next request.

## Prepare and apply storage

The administrator-managed local PV is separate from application resources in `persistentvolume/`. Adjust its node name and host path to match your cluster. Prepare the directory on that node:

```sh
sudo mkdir -p /tmp/todo-app-images
sudo chown 65532:65532 /tmp/todo-app-images
```

Apply the PV first, then the claim, Deployment, and Service:

```sh
kubectl apply -f persistentvolume/
kubectl apply -f manifests/
```

## Build and publish

From this directory:

```sh
docker build -t alejosan/todo-app:1.12 app/
docker push alejosan/todo-app:1.12
kubectl rollout restart deployment/todo-app
```

Access it locally:

```sh
kubectl port-forward service/todo-app 8080:80
```

Open `http://localhost:8080/`.

## Test cache persistence and expiry

The image cache is written to the PVC. To verify it survives a restart, request `/image`, delete the pod, wait for the replacement, and request `/image` again; it should be the same image until its persisted 10-minute TTL expires.

For a quick expiry test, restart the deployment with `IMAGE_URL` set to a local test endpoint or manually adjust `/data/state.json` in a temporary test pod so `fetched_at` is more than 10 minutes old. The first request serves the cached image; the next request triggers the refresh.

The optional `POST /shutdown` endpoint is disabled by default. To test a container restart, temporarily set `ENABLE_SHUTDOWN=true` on the Deployment, then send:

```sh
kubectl set env deployment/todo-app ENABLE_SHUTDOWN=true
kubectl port-forward service/todo-app 8080:80
curl -X POST http://localhost:8080/shutdown
```

Kubernetes restarts the container, while the image and expiry state remain on the PVC. Disable the endpoint again after testing:

```sh
kubectl set env deployment/todo-app ENABLE_SHUTDOWN-
```
