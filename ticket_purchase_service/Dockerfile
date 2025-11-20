FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o ticket_purchase_service .

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/ticket_purchase_service .

CMD ["./ticket_purchase_service"]