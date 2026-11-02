# Ping-pong and Log output with persistent storage

The Ping-pong and Log output pods share a PersistentVolumeClaim. Ping-pong persists its total request count in `/data/pingpong-count`; Log output serves the latest timestamp/UUID entry plus that count from `GET /`.

## Prepare the local PersistentVolume

The administrator-managed local PV is in `persistentvolume/`, separate from the application manifests. This example targets the `k3d-k3s-default-agent-0` node and uses `/tmp/kube`; adjust the PV's node hostname and path to match your cluster. On that node, prepare the directory for the containers' non-root user:

```sh
sudo mkdir -p /tmp/kube
sudo chown 65532:65532 /tmp/kube
```

Apply the PV first, then the claim and application resources:

```sh
kubectl apply -f persistentvolume/
kubectl apply -f manifests/
```

The local PV uses `ReadWriteOnce`; both pods are scheduled to the PV's node and can share it there. Keep each Deployment at one replica so request counts are not independently updated by multiple Ping-pong instances.

## Build and publish the images

From this directory:

```sh
docker build -t alejosan/pingpong:1.11 app_pingpong/
docker push alejosan/pingpong:1.11

docker build -t alejosan/log-output:1.11 app2_logoutput/
docker push alejosan/log-output:1.11
```

## Access the applications

Use the Ingress if configured, or forward the services locally:

```sh
kubectl port-forward service/log-output 8081:80
```

Open `http://localhost:8081/` to see the latest log entry and request count. Ping-pong is available at `/pingpong` through the Ingress, or locally with:

```sh
kubectl port-forward service/ping-pong 8082:80
curl http://localhost:8082/pingpong
```
