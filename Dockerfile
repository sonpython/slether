FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o slether-server .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/slether-server .
COPY client/ ./client/

# Override static dir to look in /app/client
ENV SLETHER_STATIC_DIR=./client

EXPOSE 8080
CMD ["./slether-server"]
