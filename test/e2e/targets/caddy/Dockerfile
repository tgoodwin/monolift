FROM golang:1.25-alpine AS build

WORKDIR /src
COPY evaluation/caddy/go.mod evaluation/caddy/go.sum ./
RUN go mod download
COPY evaluation/caddy/ ./
RUN go build -o /out/caddy ./cmd/caddy

FROM alpine:3.20
RUN adduser -D -H caddy
COPY --from=build /out/caddy /usr/bin/caddy
COPY test/e2e/targets/caddy/static/ /srv/static/
USER caddy
EXPOSE 8080
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
