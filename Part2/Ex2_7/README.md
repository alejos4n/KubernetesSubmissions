# Log output and Ping-pong with PostgreSQL

Log output and Ping-pong run in separate pods in the `exercises` namespace. Ping-pong persists its counter in a one-replica PostgreSQL StatefulSet backed by a 1 GiB PersistentVolumeClaim. The database is exposed only inside the namespace. Ping-pong waits for PostgreSQL before starting, and creates the counter table on startup. The Log output ConfigMap provides `information.txt` as a mounted file and `MESSAGE` as an environment variable. On each `GET /`, Log output prints both values, then its timestamped UUID line and the persisted pong count. `GET /pingpong` increments and displays the count; `GET /pings` reports it without incrementing.

Exercise applications use the shared `exercises` namespace. Course material can continue to use the default namespace, and the project uses its own namespace.

Build and push both images from this directory:

```sh
docker build -t alejosan/log-output:2.2 app/
docker push alejosan/log-output:2.2

docker build -t alejosan/pingpong:2.2 pingpong/
docker push alejosan/pingpong:2.2
```

Create the shared namespace, then deploy the database, apps, services, and Ingress:

```sh
kubectl apply -f manifests/namespace.yaml
kubectl apply -f manifests/
```

Check that PostgreSQL is ready before the Ping-pong pod:

```sh
kubectl get pods -n exercises
kubectl logs -n exercises postgres-0
```

If needed, forward the services locally:

```sh
kubectl port-forward -n exercises service/log-output 8081:80
kubectl port-forward -n exercises service/ping-pong 8082:80
```

Browse to `http://localhost:8081/` for the log output and `http://localhost:8082/pingpong` to increment the pong count. The Ingress routes `/` to Log output and `/pingpong` to Ping-pong.

The exercise manifest includes a development-only PostgreSQL password in `manifests/postgres-secret.yaml`. Replace it with a suitably managed secret for any non-local deployment. The counter data remains in the StatefulSet's claim across pod restarts; deleting that claim deletes the stored counter.
