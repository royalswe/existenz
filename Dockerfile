# Stage 1: Build the Go application
FROM golang:1.23-alpine AS builder
WORKDIR /app
# Copy go.mod, go.sum and server files
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o main .


# Stage 2: Create the final image
FROM alpine:latest
WORKDIR /app

# Install Chromium and necessary dependencies
RUN apk add --no-cache chromium nss freetype freetype-dev harfbuzz ca-certificates ttf-freefont

# Copy the binary and JSON file
COPY --from=builder /app/main .
COPY links.json .

# Copy static UI files
COPY ui/ ./ui

# Expose the port
EXPOSE 8081

# Set environment variables for Chromium
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_PATH=/usr/lib/chromium/

# Run the Go application
CMD ["./main"]
