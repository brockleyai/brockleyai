FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o api-test-server .

FROM alpine:3.20
COPY --from=builder /app/api-test-server /usr/local/bin/
EXPOSE 9092
CMD ["api-test-server"]
