# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /opentide ./cmd/opentide
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /tide-cli ./cmd/tide-cli

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 1000 opentide

COPY --from=builder /opentide /usr/local/bin/opentide
COPY --from=builder /tide-cli /usr/local/bin/tide-cli

USER opentide
EXPOSE 8080

ENTRYPOINT ["opentide"]
