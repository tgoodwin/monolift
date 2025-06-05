FROM golang:1.23-alpine AS builder

LABEL stage="builder"

WORKDIR /app

COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go mod verify

# Copy the source code into the container
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /monolith-server .

# Stage 2: Create the runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the pre-built binary from the builder stage
COPY --from=builder /monolith-server .

EXPOSE 8080

# Command to run the executable
CMD ["./monolith-server"]