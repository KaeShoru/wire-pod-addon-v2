# syntax=docker/dockerfile:1

# Pinned multi-architecture upstream image. The digest contains linux/amd64 and
# linux/arm64 manifests. This avoids compiling Go, Vosk or native libraries on
# the Home Assistant host.
FROM ghcr.io/kercre123/wire-pod@sha256:b8ac0d5593704592eacedd293a864e0504f4974b44e64a4c56a6fae19857fb94

# The upstream start script launches the server as a child of PID 1. Replace
# those foreground launches with exec so Supervisor stop/restart signals reach
# Wire-Pod directly and the container lifecycle matches the server lifecycle.
RUN sed -Ei 's|^([[:space:]]*)\./chipper[[:space:]]*$|\1exec ./chipper|' \
        /opt/wire-pod/chipper/start.sh \
    && grep -q 'exec ./chipper' /opt/wire-pod/chipper/start.sh

ARG BUILD_VERSION
ARG BUILD_ARCH

LABEL \
    io.hass.version="${BUILD_VERSION}" \
    io.hass.type="app" \
    io.hass.arch="${BUILD_ARCH}" \
    org.opencontainers.image.title="Wire-Pod for Home Assistant" \
    org.opencontainers.image.description="Low-overhead Wire-Pod packaging for Home Assistant" \
    org.opencontainers.image.source="https://github.com/KaeShoru/wire-pod-addon-v2" \
    org.opencontainers.image.licenses="MIT"

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=5 \
    CMD curl -fsS http://127.0.0.1:8080/ok || exit 1
