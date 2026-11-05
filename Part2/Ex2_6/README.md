# Todo app and in-memory backend

The todo app keeps the persistent Lorem Picsum image cache, fetches todos from `todo-backend` when rendering the page, and submits new todos to `POST /todos`. The backend exposes `GET /todos` and `POST /todos`, stores items in memory, and starts with the sample todos from the previous exercise. New items are limited to 140 characters.

Runtime settings—including listen ports, service URLs, image URL, cache path and timing values—are supplied to both pods through the `todo-app-config` ConfigMap. Update the ConfigMap to change these settings; the app containers have no built-in runtime defaults. Since Kubernetes does not update environment variables in running containers, restart the deployments after changing the ConfigMap:

```sh
kubectl rollout restart -n project deployment/todo-app deployment/todo-backend
```

The Ingress sends `/todos` requests from the browser to the backend and all other paths (including `/` and `/image`) to the todo app. The todo app talks to the backend over the in-cluster `todo-backend` Service.

## Build and publish

From this directory:

```sh
docker build -t alejosan/todo-app:2.3 app/
docker push alejosan/todo-app:2.3

docker build -t alejosan/todo-backend:2.3 todo-backend/
docker push alejosan/todo-backend:2.3
```

## Deploy

Project resources use the dedicated `project` namespace. The image cache uses a cluster-scoped local PersistentVolume and a static StorageClass (also cluster-scoped). Adjust the node name and host path in `persistentvolume/persistentvolume.yaml` for your cluster and prepare that directory on the node. Apply the namespace, StorageClass, and PV before the application resources:

```sh
kubectl apply -f manifests/namespace.yaml
kubectl apply -f persistentvolume/storageclass.yaml
kubectl apply -f persistentvolume/persistentvolume.yaml
kubectl apply -f manifests/
```

If a previous local PV is stuck Released and the claim remains Pending, remove and recreate the PV after confirming it is safe to do so and that the image-cache data can be discarded. Then re-apply the claim and application manifests.

Access the app through the Ingress, or use:

```sh
kubectl port-forward -n project service/todo-app 8080:80
```

Open `http://localhost:8080/`. The todo app proxies `/todos` GET and POST requests to the backend, so the form also works when accessed through this port-forward.
