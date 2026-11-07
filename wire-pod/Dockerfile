# syntax=docker/dockerfile:1

# Pinned multi-architecture upstream image. The source tree and Vosk runtime in
# this image are reused to build a small patched chipper binary in CI. Nothing
# is compiled on the Home Assistant host.
FROM ghcr.io/kercre123/wire-pod@sha256:b8ac0d5593704592eacedd293a864e0504f4974b44e64a4c56a6fae19857fb94 AS upstream

FROM golang:1.22.4-bookworm AS chipper-builder

ARG BUILD_VERSION=dev

ENV DEBIAN_FRONTEND=noninteractive \
    CGO_ENABLED=1 \
    LD_LIBRARY_PATH=/opt/vosk/libvosk

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libasound2-dev \
        libopus-dev \
        libopusfile-dev \
        libsodium-dev \
        patch \
    && rm -rf /var/lib/apt/lists/*

COPY --from=upstream /opt/wire-pod /src
COPY --from=upstream /opt/vosk /opt/vosk
COPY patches/native_russian.go /src/chipper/pkg/wirepod/ttr/native_russian.go
COPY patches/native_russian_test.go /src/chipper/pkg/wirepod/ttr/native_russian_test.go
COPY patches/vector_tts.go /src/chipper/pkg/wirepod/ttr/vector_tts.go
COPY patches/vector_vocoder.go /src/chipper/pkg/wirepod/ttr/vector_vocoder.go
COPY patches/vector_vocoder_sinctable.go /src/chipper/pkg/wirepod/ttr/vector_vocoder_sinctable.go
COPY patches/vector_vocoder_test.go /src/chipper/pkg/wirepod/ttr/vector_vocoder_test.go
COPY patches/vector_tts_amplify_test.go /src/chipper/pkg/wirepod/ttr/vector_tts_amplify_test.go
COPY patches/native-russian-stream.patch /tmp/native-russian-stream.patch

WORKDIR /src
RUN patch -p1 </tmp/native-russian-stream.patch \
    && cd chipper \
    && CGO_CFLAGS="-I/opt/vosk/libvosk" \
       CGO_LDFLAGS="-L/opt/vosk/libvosk -lvosk -lopus -lopusfile -lsodium -lasound -ldl -lpthread" \
       go test ./pkg/wirepod/ttr \
    && CGO_CFLAGS="-I/opt/vosk/libvosk" \
       CGO_LDFLAGS="-L/opt/vosk/libvosk -lvosk -lopus -lopusfile -lsodium -lasound -ldl -lpthread" \
       go build -tags "nolibopusfile" \
       -ldflags "-s -w -X github.com/kercre123/wire-pod/chipper/pkg/vars.CommitSHA=kaeshoru-${BUILD_VERSION}" \
       -o /tmp/chipper ./cmd/vosk

FROM upstream

COPY --from=chipper-builder /tmp/chipper /opt/wire-pod/chipper/chipper

RUN apt-get update \
    && apt-get install -y --no-install-recommends jq nginx-core \
    && rm -rf /var/lib/apt/lists/*

# The upstream start script launches the server as a child of PID 1. Replace
# those foreground launches with exec so Supervisor stop/restart signals reach
# Wire-Pod directly and the container lifecycle matches the server lifecycle.
RUN sed -Ei 's|^([[:space:]]*)\./chipper[[:space:]]*$|\1exec ./chipper|' \
        /opt/wire-pod/chipper/start.sh \
    && grep -q 'exec ./chipper' /opt/wire-pod/chipper/start.sh

COPY nginx-ingress.conf /etc/nginx/nginx.conf
COPY run.sh /usr/local/bin/run-wire-pod
RUN chmod 0755 /usr/local/bin/run-wire-pod

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

CMD ["/usr/local/bin/run-wire-pod"]
