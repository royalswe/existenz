# Stage 1: Build the Go application
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache build-base

# Copy go.mod and go.sum to cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Stage 2: Create the final image
FROM alpine:latest
WORKDIR /app

# Install Chromium and necessary dependencies
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    freetype-dev \
    harfbuzz \
    ca-certificates \
    ttf-freefont

# Set environment variables for Chromium
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_PATH=/usr/lib/chromium/

# Copy the built binary
COPY --from=builder /app/main .

# Create a temporary directory
RUN mkdir tmp

# Copy the JSON file and static UI files
COPY links.json .
COPY ui/ ./ui

# Expose the application port
EXPOSE 8081

# Run the Go application
CMD ["./main"]
