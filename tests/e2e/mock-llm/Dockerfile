FROM golang:1.25-alpine AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o mock-llm .

FROM alpine:3.19
COPY --from=build /app/mock-llm /usr/local/bin/
COPY scripts.json /app/scripts.json
WORKDIR /app
ENV SCRIPT_FILE=/app/scripts.json
EXPOSE 9091
CMD ["mock-llm"]
