FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY demo/monolith ./demo/monolith

RUN CGO_ENABLED=0 GOOS=linux go build -a -o /app/monolith-server ./demo/monolith/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

# A non-root user is a good security practice
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

WORKDIR /home/appuser/

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/monolith-server .

EXPOSE 8080

# Command to run the executable
CMD ["./monolith-server"]