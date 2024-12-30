# Stage 1: Build the Go application
FROM golang:1.23-alpine AS builder
WORKDIR /app
# Copy go.mod, go.sum and server files
COPY / ./
COPY go.mod go.sum ./

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o main .


# Stage 2: Create the final image
FROM alpine:latest
WORKDIR /app

# Copy the binary
COPY --from=builder /app/main .
COPY ui/ ./ui

# Expose the port
EXPOSE 8081

# Run the Go application
CMD ["./main"]
