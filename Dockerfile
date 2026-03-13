# Stage 1: Build the binary
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Download dependencies first (cached layer unless go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o loginapp ./cmd/main.go

# Stage 2: Minimal runtime image
FROM alpine:3.19

# ca-certificates needed for TLS, tzdata for timezone support
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/loginapp /usr/local/bin/loginapp

ENTRYPOINT ["loginapp"]
