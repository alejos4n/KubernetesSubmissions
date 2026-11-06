# Todo app and PostgreSQL backend

The todo app keeps the persistent Lorem Picsum image cache, fetches todos from `todo-backend` when rendering the page, and submits new todos to `POST /todos`. The backend exposes `GET /todos` and `POST /todos` and saves todos in PostgreSQL. The database runs as a one-replica StatefulSet with a persistent volume claim. On first startup, the backend creates the table and seeds the sample todos; later restarts retain the saved todos. Todos are limited to 140 characters in both the app and backend.

The backend logs each submitted todo to stdout, including over-limit todos rejected with HTTP 400. Kubernetes collects these container logs for Loki/Grafana. To verify the limit against the backend, forward the service and submit a 141-character todo:

```sh
kubectl port-forward -n project service/todo-backend 8083:80
curl -i -X POST http://localhost:8083/todos \
  -H 'Content-Type: application/json' \
  --data "$(printf '{\"todo\":\"%0141d\"}' 0)"
```

The request should return `400 Bad Request` with `todo must be 140 characters or fewer`. In Grafana, open **Explore**, select **Loki**, and query `{namespace="project", container="todo-backend"} |= "todo rejected"` to find the logged rejection (including the message and measured length).

Runtime settings—including listen ports, service URLs, image URL, cache path, timing values and PostgreSQL connection settings—are supplied through the `todo-app-config` ConfigMap. The PostgreSQL password is supplied through the `todo-postgres-credentials` Secret. Update configuration as needed; the app containers have no built-in runtime defaults. Since Kubernetes does not update environment variables in running containers, restart the deployments after changing the ConfigMap or Secret:

```sh
kubectl rollout restart -n project deployment/todo-app deployment/todo-backend statefulset/todo-postgres
```

The Ingress sends `/todos` requests from the browser to the backend and all other paths (including `/` and `/image`) to the todo app. The todo app talks to the backend over the in-cluster `todo-backend` Service.

## Hourly Wikipedia reminder

The `random-wikipedia-todo` CronJob runs at the start of every hour. It reads the `Location` header from Wikipedia's `Special:Random` redirect, then POSTs a todo in the form `Read https://en.wikipedia.org/wiki/...` to the backend using `TODO_BACKEND_URL` from `todo-app-config`.

After applying the manifests, check the schedule and create a one-off test job with:

```sh
kubectl get cronjob -n project random-wikipedia-todo
kubectl create job -n project --from=cronjob/random-wikipedia-todo random-wikipedia-todo-test
kubectl logs -n project job/random-wikipedia-todo-test
```

Each successful run adds its chosen article URL to the todo list.

## Build and publish

From this directory:

```sh
docker build -t alejosan/todo-app:2.3 app/
docker push alejosan/todo-app:2.3

docker build -t alejosan/todo-backend:2.5 todo-backend/
docker push alejosan/todo-backend:2.5
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

The database has its own 1 GiB claim created by the StatefulSet. The database password in `manifests/todo-postgres-secret.yaml` is for local exercise use only; replace it with a suitably managed secret outside a local development cluster. Deleting the database claim permanently deletes the saved todos.

Access the app through the Ingress, or use:

```sh
kubectl port-forward -n project service/todo-app 8080:80
```

Open `http://localhost:8080/`. The todo app proxies `/todos` GET and POST requests to the backend, so the form also works when accessed through this port-forward.
