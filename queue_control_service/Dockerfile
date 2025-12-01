FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN go build -o queue_control_service .

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/queue_control_service .

CMD ["./queue_control_service"]