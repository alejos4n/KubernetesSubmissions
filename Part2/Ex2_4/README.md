# Todo app and in-memory backend

The todo app keeps the persistent Lorem Picsum image cache, fetches todos from `todo-backend` when rendering the page, and submits new todos to `POST /todos`. The backend exposes `GET /todos` and `POST /todos`, stores items in memory, and starts with the sample todos from the previous exercise. New items are limited to 140 characters.

The Ingress sends `/todos` requests from the browser to the backend and all other paths (including `/` and `/image`) to the todo app. The todo app talks to the backend over the in-cluster `todo-backend` Service.

## Build and publish

From this directory:

```sh
docker build -t alejosan/todo-app:2.2 app/
docker push alejosan/todo-app:2.2

docker build -t alejosan/todo-backend:2.2 todo-backend/
docker push alejosan/todo-backend:2.2
```

## Deploy

Project resources use the dedicated `project` namespace. The image cache continues to use the local PersistentVolume, which is cluster-scoped. Adjust the node name and host path in `persistentvolume/persistentvolume.yaml` for your cluster, prepare that directory on the node, and create the namespace before applying the PV and application resources:

```sh
kubectl apply -f namespace.yaml
kubectl apply -f persistentvolume/
kubectl apply -f manifests/
```

If a previous local PV is stuck Released and the claim remains Pending, remove and recreate the PV after confirming it is safe to do so and that the image-cache data can be discarded. Then re-apply the claim and application manifests.

Access the app through the Ingress, or use:

```sh
kubectl port-forward -n project service/todo-app 8080:80
```

Open `http://localhost:8080/`. The todo app proxies `/todos` GET and POST requests to the backend, so the form also works when accessed through this port-forward.
