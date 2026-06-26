# Stage 1: Build web-ui React app
FROM node:20-alpine AS web-builder
WORKDIR /web-ui
COPY web-ui/package*.json ./
RUN npm ci
COPY web-ui/. .
RUN npm run build

# Stage 2: Build Go binaries
FROM golang:1.21-alpine AS go-builder

ARG TARGETARCH
ENV TARGETARCH=${TARGETARCH:-arm64}

WORKDIR /build

RUN apk add --no-cache git binutils

# Download and extract sing-box
RUN wget -qO- "https://github.com/shtorm-7/sing-box-extended/releases/download/v1.13.12-extended-2.4.1/sing-box-1.13.12-extended-2.4.1-linux-${TARGETARCH}.tar.gz" \
    | tar xz -C /tmp \
    && mv /tmp/sing-box-1.13.12-extended-2.4.1-linux-${TARGETARCH}/sing-box /usr/local/bin/ \
    && rm -rf /tmp/sing-box-* \
    && strip /usr/local/bin/sing-box

# Build API server
RUN mkdir -p /build/api_server
WORKDIR /build/api_server
COPY sing-box/go.mod .
COPY sing-box/api_server.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o api_server . \
    && strip api_server

# Build web-ui server
RUN mkdir -p /build/web_ui
WORKDIR /build/web_ui
COPY --from=web-builder /web-ui/dist ./dist
COPY web-ui/go.mod .
COPY web-ui/server.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o web_ui_server . \
    && strip web_ui_server

# Copy CA certs
RUN apk add --no-cache ca-certificates

# Final stage — minimal alpine
FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget jq iproute2 tzdata curl

# Copy binaries
COPY --from=go-builder /usr/local/bin/sing-box /sing-box
COPY --from=go-builder /build/api_server/api_server /api_server
COPY --from=go-builder /build/web_ui/web_ui_server /web_ui_server
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy entrypoint
COPY sing-box/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /sing-box /api_server /web_ui_server

EXPOSE 9090/tcp
EXPOSE 20123/tcp
EXPOSE 11501/tcp

ENTRYPOINT ["/entrypoint.sh"]
