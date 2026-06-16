FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git

# Copy and build
COPY go.mod .
COPY server.go .
COPY public/ ./public/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o web-ui .

# Final stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary
COPY --from=builder /build/web-ui .

EXPOSE 8080

ENV PORT=8080

CMD ["./web-ui"]
