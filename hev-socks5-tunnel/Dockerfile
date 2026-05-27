# hev-socks5-tunnel container for MikroTik
# Downloads binary at build time from GitHub Releases

ARG ALPINE_VERSION=3.23

FROM --platform=$BUILDPLATFORM alpine:${ALPINE_VERSION}

ARG TARGETPLATFORM

RUN apk add --no-cache curl tzdata iproute2 openrc \
    && mkdir -p /usr/local/bin /etc/hev-socks5-tunnel \
    && if [ "$TARGETPLATFORM" = "linux/amd64" ]; then \
         ARCH="x86_64"; \
       elif [ "$TARGETPLATFORM" = "linux/arm64" ]; then \
         ARCH="arm64"; \
       elif [ "$TARGETPLATFORM" = "linux/arm/v7" ]; then \
         ARCH="arm32v7hf"; \
       else \
         echo "Unsupported platform: $TARGETPLATFORM" && exit 1; \
       fi \
    && echo "Downloading hev-socks5-tunnel for linux-${ARCH}..." \
    && curl -fsSL -o /usr/local/bin/hev-socks5-tunnel \
       "https://github.com/heiher/hev-socks5-tunnel/releases/latest/download/hev-socks5-tunnel-linux-${ARCH}" \
    && chmod +x /usr/local/bin/hev-socks5-tunnel

# Disable tty in inittab for container compatibility
RUN sed -i 's/^tty/#tty/' /etc/inittab 2>/dev/null || true

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
RUN sed -i 's/\r$//' /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/sbin/init"]
