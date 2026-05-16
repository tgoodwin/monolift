FROM golang:1.25 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN mkdir -p /out && CGO_ENABLED=0 go build -mod=mod -o /out/pocketbase ./examples/base

FROM alpine:3.20
COPY --from=builder /out/pocketbase /pocketbase
COPY --from=builder /src/tests/data /seed_pb_data
EXPOSE 8090
ENTRYPOINT ["/pocketbase"]
