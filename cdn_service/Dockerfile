FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o cdn_service .

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/cdn_service .
COPY --from=builder /app/static ./static

CMD ["./cdn_service"]