FROM golang:1.22-alpine AS builder
LABEL authors="pavelmikhailov"

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o /app/monitor pafaul/telegram-http-monitor

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /bin/telegram-http-monitor /bin/telegram-http-monitor
#CMD /bin/sh -c -- "while true; do sleep 30; done;"
CMD ["/app/monitor"]