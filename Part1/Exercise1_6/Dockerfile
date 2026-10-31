FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -o /todo-app .

FROM scratch
COPY --from=build /todo-app /todo-app
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/todo-app"]
