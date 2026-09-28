# --platform=$BUILDPLATFORM: compile natively on the builder and cross-compile
# for the target (Go needs no emulation for that), so a multi-arch
# `docker buildx build --platform linux/amd64,linux/arm64` doesn't run the
# Go compiler under QEMU. Plain `docker build` / `docker compose build`
# behave exactly as before.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/traccia ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 1000 traccia
COPY --from=build /out/traccia /usr/local/bin/traccia
WORKDIR /app
RUN chown traccia:traccia /app
# ./plugins is bind-mounted at runtime (see docker-compose.yml) — this user
# only needs to read those .js files, which works as long as they keep the
# host's default world-readable permissions.
USER traccia
EXPOSE 8080
ENTRYPOINT ["traccia"]
