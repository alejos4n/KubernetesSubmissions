# Log output and Ping-pong over HTTP

Log output and Ping-pong run in separate pods in the `exercises` namespace with no shared volume. The Log output ConfigMap provides `information.txt` as a mounted file and `MESSAGE` as an environment variable. On each `GET /`, Log output prints both values, then its timestamped UUID line and the pong count. `GET /pingpong` increments and displays the count; `GET /pings` reports it without incrementing.

Exercise applications use the shared `exercises` namespace. Course material can continue to use the default namespace, and the project uses its own namespace.

Build and push both images from this directory:

```sh
docker build -t alejosan/log-output:2.2 app/
docker push alejosan/log-output:2.2

docker build -t alejosan/pingpong:2.1 pingpong/
docker push alejosan/pingpong:2.1
```

Create the shared namespace, then deploy both apps, services, and Ingress:

```sh
kubectl apply -f namespace.yaml
kubectl apply -f manifests/
```

If needed, forward the services locally:

```sh
kubectl port-forward -n exercises service/log-output 8081:80
kubectl port-forward -n exercises service/ping-pong 8082:80
```

Browse to `http://localhost:8081/` for the log output and `http://localhost:8082/pingpong` to increment the pong count. The Ingress routes `/` to Log output and `/pingpong` to Ping-pong.
