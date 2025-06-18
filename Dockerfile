FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY . .

RUN go build -o proxy main.go

FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/proxy /usr/local/bin/proxy

EXPOSE 9000
ENTRYPOINT ["/usr/local/bin/proxy"]
