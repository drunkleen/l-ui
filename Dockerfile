# ========================================================
# Stage: binary-select — pick the right binary for TARGETARCH.
# All arch binaries are in the build context; this stage
# copies only the one matching the target platform.
# ========================================================
FROM alpine:3.21 AS binary-select
ARG TARGETARCH
ARG TARGETVARIANT
COPY l-ui-hub-* /tmp/
RUN f="l-ui-hub-${TARGETARCH}${TARGETVARIANT}" && \
    [ -f "/tmp/$f" ] && mv "/tmp/$f" /l-ui-hub && rm -f /tmp/l-ui-hub-* || \
    { echo "no binary for ${TARGETARCH}${TARGETVARIANT}"; exit 1; }

# ========================================================
# Stage: final — l-ui hub image
# ========================================================
FROM alpine:3.21

ARG TARGETARCH
ARG TARGETVARIANT

ENV TZ=Asia/Tehran
WORKDIR /app

RUN apk add --no-cache --update \
  ca-certificates \
  tzdata \
  fail2ban \
  bash \
  curl \
  wget \
  openssl \
  unzip \
  && addgroup -g 1000 -S l-ui \
  && adduser -u 1000 -S l-ui -G l-ui \
  && mkdir -p /etc/l-ui /var/log/l-ui \
  && chown -R l-ui:l-ui /etc/l-ui /var/log/l-ui

COPY --from=binary-select /l-ui-hub /app/l-ui
COPY DockerEntrypoint.sh DockerInit.sh /app/
COPY l-ui.sh /usr/bin/l-ui
COPY hub/web/translation /app/hub/web/translation

RUN case "${TARGETARCH}${TARGETVARIANT}" in \
      armv7|armv6) DOCKER_ARCH="${TARGETARCH}${TARGETVARIANT}" ;; \
      *)           DOCKER_ARCH="$TARGETARCH" ;; \
    esac && /app/DockerInit.sh "$DOCKER_ARCH"

RUN rm -f /etc/fail2ban/jail.d/alpine-ssh.conf \
  && cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local \
  && sed -i "s/^\[ssh\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
  && sed -i "s/^\[sshd\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
  && sed -i "s/#allowipv6 = auto/allowipv6 = auto/g" /etc/fail2ban/fail2ban.conf

RUN chmod +x /app/DockerEntrypoint.sh /app/l-ui /usr/bin/l-ui \
  && chown -R l-ui:l-ui /app

USER l-ui

LABEL org.opencontainers.image.title="L-UI" \
      org.opencontainers.image.description="Hub for managing remote VPS nodes and Xray instances" \
      org.opencontainers.image.source="https://github.com/drunkleen/l-ui"

ENV LUI_IN_DOCKER="true"
ENV LUI_MAIN_FOLDER="/app"
ENV LUI_ENABLE_FAIL2BAN="true"
ENV LUI_DB_TYPE=""
ENV LUI_DB_DSN=""
EXPOSE 2053
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:2053/healthz || exit 1
VOLUME [ "/etc/l-ui" ]
CMD [ "run" ]
ENTRYPOINT [ "/app/DockerEntrypoint.sh" ]
