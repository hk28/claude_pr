FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o prman .

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/prman .
COPY static/ static/

EXPOSE 8080
VOLUME ["/app/config", "/app/data", "/inbox", "/outbox"]

ENTRYPOINT ["./prman"]
CMD ["-config", "/app/config", "-data", "/app/data", "-addr", ":8080"]
