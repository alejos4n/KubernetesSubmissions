# Log output application

The application runs as two containers in one pod:

- `log-writer` generates a random UUID when it starts and appends it with a UTC timestamp to `/data/log.txt` immediately and every five seconds.
- `log-server` serves the complete file contents from `GET /` on port 8080.

Both containers share an `emptyDir` volume. The HTTP server mounts it read-only.

Build and push the image:

```sh
docker build -t alejosan/log-output:1.10 -f app/Dockerfile .
docker push alejosan/log-output:1.10
```

Deploy the application:

```sh
kubectl apply -f manifests/
```

To access the endpoint locally:

```sh
kubectl port-forward service/log-output 8080:80
curl http://localhost:8080/
```
