# Xray SOCKS proxy container for MikroTik
# Downloads Xray binary at build time from GitHub Releases

ARG ALPINE_VERSION=3.23

FROM --platform=$BUILDPLATFORM alpine:${ALPINE_VERSION}

ARG TARGETPLATFORM

RUN apk add --no-cache curl unzip tzdata \
    && mkdir -p /usr/local/bin \
    && if [ "$TARGETPLATFORM" = "linux/amd64" ]; then \
         ARCH="64"; \
       elif [ "$TARGETPLATFORM" = "linux/arm64" ]; then \
         ARCH="arm64-v8a"; \
       elif [ "$TARGETPLATFORM" = "linux/arm/v7" ]; then \
         ARCH="arm32-v7a"; \
       else \
         echo "Unsupported platform: $TARGETPLATFORM" && exit 1; \
       fi \
    && echo "Downloading Xray for Linux ${ARCH}..." \
    && curl -fsSL -o /tmp/xray.zip "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-${ARCH}.zip" \
    && unzip -o /tmp/xray.zip -d /usr/local/bin/ \
    && rm -f /tmp/xray.zip \
    && chmod +x /usr/local/bin/xray

EXPOSE 10800

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
RUN sed -i 's/\r$//' /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
