#!/usr/bin/env bash
set -Eeuo pipefail

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
