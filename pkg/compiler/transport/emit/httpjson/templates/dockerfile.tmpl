FROM golang:1.25 AS builder

WORKDIR /src/extracted-cleanpath
COPY ./extracted-cleanpath /src/extracted-cleanpath
COPY ./upstream /src/upstream
RUN go build -mod=mod -o /out/extracted .

FROM gcr.io/distroless/static
COPY --from=builder /out/extracted /extracted
EXPOSE 8081
HEALTHCHECK --interval=5s --timeout=2s --retries=12 CMD ["/extracted", "-healthcheck"]
ENTRYPOINT ["/extracted"]
