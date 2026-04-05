# --- Dev stage: includes source mount for hot-reload ---
FROM golang:1.24-alpine AS dev
RUN apk add --no-cache curl gettext
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o brockley-server ./cmd/server
CMD ["./brockley-server"]

# --- Builder stage ---
FROM golang:1.24-alpine AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o brockley-server ./cmd/server

# --- Production stage: minimal image ---
FROM alpine:3.20 AS production
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/brockley-server /usr/local/bin/
EXPOSE 8000
CMD ["brockley-server"]
