# Log output and Ping-pong over HTTP

Log output and Ping-pong run in separate pods with no shared volume. On each `GET /`, Log output makes an HTTP `GET http://ping-pong/pings` request and returns its timestamped UUID line together with the pong count. `GET /pingpong` increments and displays the count; `GET /pings` reports it without incrementing.

Build and push both images from this directory:

```sh
docker build -t alejosan/log-output:2.1 app/
docker push alejosan/log-output:2.1

docker build -t alejosan/pingpong:2.1 pingpong/
docker push alejosan/pingpong:2.1
```

Deploy both apps, services, and Ingress:

```sh
kubectl apply -f manifests/
```

If needed, forward the services locally:

```sh
kubectl port-forward service/log-output 8081:80
kubectl port-forward service/ping-pong 8082:80
```

Browse to `http://localhost:8081/` for the log output and `http://localhost:8082/pingpong` to increment the pong count. The Ingress routes `/` to Log output and `/pingpong` to Ping-pong.
