FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -o /todo-backend .

FROM scratch
COPY --from=build /todo-backend /todo-backend
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/todo-backend"]
