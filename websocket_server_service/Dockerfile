FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o websocket_service .

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/websocket_service .

CMD ["./websocket_service"]