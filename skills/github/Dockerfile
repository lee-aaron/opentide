FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY main.go .
COPY go.mod .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /skill .

FROM alpine:3.21
RUN adduser -D -u 1000 skill
COPY --from=builder /skill /skill
USER skill
ENTRYPOINT ["/skill"]
