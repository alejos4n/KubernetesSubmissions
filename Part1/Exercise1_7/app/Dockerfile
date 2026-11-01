FROM golang:1.24-alpine AS build

WORKDIR /src
COPY app/go.mod app/main.go ./
RUN CGO_ENABLED=0 go build -o /log-output .

FROM scratch
COPY --from=build /log-output /log-output
USER 65532:65532
ENTRYPOINT ["/log-output"]
