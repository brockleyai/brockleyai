FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod main.go tools.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o mcp-test-server .

FROM alpine:3.20
COPY --from=builder /app/mcp-test-server /usr/local/bin/
EXPOSE 9090
CMD ["mcp-test-server"]
