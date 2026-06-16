FROM alpine:3.21

ARG TARGETARCH
ENV TARGETARCH=${TARGETARCH:-arm64}

RUN apk add --no-cache wget jq ca-certificates \
    && wget -qO- "https://github.com/shtorm-7/sing-box-extended/releases/download/v1.13.12-extended-2.4.1/sing-box-1.13.12-extended-2.4.1-linux-${TARGETARCH}.tar.gz" \
    | tar -xzf - \
    && cp "sing-box-1.13.12-extended-2.4.1-linux-${TARGETARCH}/sing-box" /usr/local/bin/sing-box \
    && rm -rf "sing-box-1.13.12-extended-2.4.1-linux-${TARGETARCH}"* \
    && chmod +x /usr/local/bin/sing-box \
    && sing-box version

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
