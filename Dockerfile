FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/traccia ./cmd/api

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
