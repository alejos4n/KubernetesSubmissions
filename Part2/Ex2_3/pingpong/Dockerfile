FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -o /pingpong .

FROM scratch
COPY --from=build /pingpong /pingpong
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/pingpong"]
