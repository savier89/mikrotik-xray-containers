FROM golang:1.21-alpine AS builder

ARG TARGETARCH
ENV TARGETARCH=${TARGETARCH:-arm64}

WORKDIR /build

# Install dependencies for build
RUN apk add --no-cache git

# Download and extract sing-box
RUN wget -qO- "https://github.com/shtorm-7/sing-box-extended/releases/download/v1.13.12-extended-2.4.1/sing-box-1.13.12-extended-2.4.1-linux-${TARGETARCH}.tar.gz" \
    | tar xz -C /tmp \
    && mv /tmp/sing-box-1.13.12-extended-2.4.1-linux-${TARGETARCH}/sing-box /usr/local/bin/ \
    && rm -rf /tmp/sing-box-* \
    && chmod +x /usr/local/bin/sing-box \
    && sing-box version

# Copy and build API server
COPY go.mod .
COPY api_server.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o api_server .

# Final stage
FROM alpine:3.21

RUN apk add --no-cache wget jq ca-certificates

# Copy binaries
COPY --from=builder /usr/local/bin/sing-box /usr/local/bin/sing-box
COPY --from=builder /build/api_server /usr/local/bin/api_server

# Copy entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /usr/local/bin/sing-box /usr/local/bin/api_server

EXPOSE 9090/tcp
EXPOSE 20123/tcp

ENTRYPOINT ["/entrypoint.sh"]
