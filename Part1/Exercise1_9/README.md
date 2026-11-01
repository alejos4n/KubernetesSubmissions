## Ping-pong application

The ping-pong application responds to `GET /pingpong` with `pong 0`, `pong 1`, and so on. Its in-memory counter starts at zero and increments for each successful request; it may reset when the application restarts.

Build and push the ping-pong image:

```sh
docker build -t alejosan/exercise1-9-ping-pong:latest app/
docker push alejosan/exercise1-9-ping-pong:latest
```

Apply the manifests:

```sh
kubectl apply -f manifests/
```

The shared Ingress routes `/pingpong` to the ping-pong application and `/` to Log output. Access both applications through the Ingress controller's address.
