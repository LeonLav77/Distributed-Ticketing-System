FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o web_socket_service .

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/web_socket_service .

CMD ["./web_socket_service"]