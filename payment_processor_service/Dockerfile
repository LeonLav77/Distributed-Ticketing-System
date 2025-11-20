FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o payment_processor_service .

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/payment_processor_service .

CMD ["./payment_processor_service"]