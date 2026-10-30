#!/usr/bin/env bash
set -Eeuo pipefail

image="local/wire-pod-addon-v2:smoke"
container_name="wire-pod-addon-v2-smoke-${GITHUB_RUN_ID:-local}-$$"
volume_name="wire-pod-addon-v2-smoke-${GITHUB_RUN_ID:-local}-$$"

cleanup() {
    if docker container inspect "${container_name}" >/dev/null 2>&1; then
        echo "Wire-Pod container log:"
        docker logs "${container_name}" || true
        docker stop --time 10 "${container_name}" >/dev/null || true
        docker container rm "${container_name}" >/dev/null || true
    fi
    docker volume rm "${volume_name}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker build \
    --build-arg BUILD_ARCH=amd64 \
    --build-arg BUILD_VERSION=smoke \
    --tag "${image}" \
    wire-pod

docker run \
    --detach \
    --name "${container_name}" \
    --network host \
    --env WIREPOD_DATA_DIR=/data \
    --volume "${volume_name}:/data" \
    "${image}" >/dev/null

for attempt in $(seq 1 60); do
    if curl --fail --silent --show-error http://127.0.0.1:8080/ok >/dev/null; then
        echo "Wire-Pod became ready after ${attempt}s"
        docker stats --no-stream \
            --format 'Memory={{.MemUsage}} CPU={{.CPUPerc}}' \
            "${container_name}"
        exit 0
    fi

    if [ "$(docker inspect --format '{{.State.Running}}' "${container_name}")" != "true" ]; then
        echo "Wire-Pod stopped before becoming ready" >&2
        exit 1
    fi

    sleep 1
done

echo "Wire-Pod did not become ready within 60 seconds" >&2
exit 1
