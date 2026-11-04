#!/usr/bin/env bash
set -Eeuo pipefail

OPTIONS=/data/options.json
opt() {
    if [ -s "${OPTIONS}" ]; then
        jq -r --arg key "$1" 'if has($key) then .[$key] else empty end' "${OPTIONS}"
    fi
}

# Optional Russian TTS stays outside this lightweight app. The app streams the
# resulting 16 kHz PCM to Vector and only applies the experimental vocoder when
# it is explicitly enabled.
export VECTOR_TTS_URL="$(opt tts_url)"
export VECTOR_TTS_SPEAKER="$(opt tts_speaker)"
export VECTOR_TTS_ENGINE="$(opt tts_engine)"
export VECTOR_TTS_VARIANT="$(opt tts_variant)"
export VECTOR_TTS_SPEED="$(opt tts_speed)"
export VECTOR_TTS_PROFILE="$(opt tts_profile)"
export VECTOR_TTS_PITCH="$(opt tts_pitch)"
export VECTOR_TTS_VOLUME="$(opt tts_volume)"
export VECTOR_TTS_VOCODER="$(opt vector_vocoder)"
export VECTOR_TTS_CARRIER_HZ="$(opt vocoder_carrier_hz)"
export VECTOR_TTS_VOCODER_MIX="$(opt vocoder_mix)"

if [ -n "${VECTOR_TTS_URL}" ]; then
    echo "[wire-pod] Local Russian TTS: ${VECTOR_TTS_URL}; engine=${VECTOR_TTS_ENGINE:-silero}; variant=${VECTOR_TTS_VARIANT:-none}; speaker=${VECTOR_TTS_SPEAKER:-eugene}; profile=${VECTOR_TTS_PROFILE:-natural}; speed=${VECTOR_TTS_SPEED:-0.82}; pitch=${VECTOR_TTS_PITCH:-1.0}; volume=${VECTOR_TTS_VOLUME:-50}; experimental_vocoder=${VECTOR_TTS_VOCODER:-false}"
else
    echo "[wire-pod] Local Russian TTS is disabled; native Vector speech remains active"
fi

mkdir -p /tmp/nginx/client_body /tmp/nginx/proxy /tmp/nginx/fastcgi \
    /tmp/nginx/uwsgi /tmp/nginx/scgi

wirepod_pid=""
nginx_pid=""

shutdown() {
    trap - TERM INT
    [ -z "${wirepod_pid}" ] || kill -TERM "${wirepod_pid}" 2>/dev/null || true
    [ -z "${nginx_pid}" ] || kill -TERM "${nginx_pid}" 2>/dev/null || true
    wait || true
}
trap shutdown TERM INT

nginx -g 'daemon off;' &
nginx_pid=$!

/opt/wire-pod/chipper/start.sh &
wirepod_pid=$!

status=0
wait -n "${wirepod_pid}" "${nginx_pid}" || status=$?
shutdown
exit "${status}"
