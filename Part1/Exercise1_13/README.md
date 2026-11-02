# Todo app

The home page keeps the persistent Lorem Picsum image and adds a todo input limited to 140 characters, a non-submitting Send button, and a starter list of hardcoded todos. The button is intentionally not wired to submit yet.

Build and publish the image from this directory:

```sh
docker build -t alejosan/todo-app:1.13 app/
docker push alejosan/todo-app:1.13
```

Apply the local PersistentVolume first, after adjusting its node name and host path for the cluster. Then apply the application manifests:

```sh
kubectl apply -f persistentvolume/
kubectl apply -f manifests/
```
