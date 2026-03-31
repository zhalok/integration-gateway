FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o mock-server .

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/mock-server .

EXPOSE 9001 9002 9003

CMD ["./mock-server"]
