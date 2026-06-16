FROM alpine:3.21

ARG TARGETARCH
ENV TARGETARCH=${TARGETARCH:-arm64}

RUN apk add --no-cache wget jq ca-certificates python3 \
    && wget -qO- "https://github.com/shtorm-7/sing-box-extended/releases/download/v1.13.12-extended-2.4.1/sing-box-1.13.12-extended-2.4.1-linux-${TARGETARCH}.tar.gz" \
    | tar xz -C /tmp \
    && mv /tmp/sing-box-1.13.12-extended-2.4.1-linux-${TARGETARCH}/sing-box /usr/local/bin/ \
    && rm -rf /tmp/sing-box-* \
    && chmod +x /usr/local/bin/sing-box \
    && sing-box version

COPY entrypoint.sh /entrypoint.sh
COPY api_server.py /api_server.py
COPY tests/ /tests/

RUN chmod +x /entrypoint.sh /api_server.py /tests/*.sh

EXPOSE 9090/tcp
EXPOSE 20123/tcp

ENTRYPOINT ["/entrypoint.sh"]
