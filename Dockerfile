# syntax=docker/dockerfile:1

FROM golang:1.26.7-alpine3.24 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY web ./web
RUN CGO_ENABLED=0 go test ./... && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/pool .

FROM alpine:3.24

RUN addgroup -S -g 10001 pool && \
    adduser -S -D -H -u 10001 -G pool pool && \
    mkdir -p /data && \
    chown pool:pool /data

WORKDIR /app
COPY --from=build --chown=pool:pool /out/pool /app/pool
COPY --from=build --chown=pool:pool /src/web /app/web

USER pool
ENV APP_PASSWORD_FILE=/run/secrets/app_password \
    BIND_ADDRESS=0.0.0.0 \
    DATA_PATH=/data/pool.db \
    PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/ping >/dev/null || exit 1

ENTRYPOINT ["/app/pool"]
