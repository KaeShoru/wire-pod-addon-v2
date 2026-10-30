# Wire-Pod for Home Assistant

A lightweight Home Assistant app (formerly add-on) packaging of
[Wire-Pod](https://github.com/kercre123/wire-pod) for Anki Vector.

The app does **not** compile Wire-Pod, Go, or Vosk on the Home Assistant host.
Release installations download a pre-built multi-architecture image. This keeps
installation predictable on Raspberry Pi and other memory-constrained systems.

## Supported systems

- `aarch64` (64-bit Raspberry Pi 4/5 and other ARM64 Home Assistant systems)
- `amd64` (x86-64 Home Assistant systems)

The official Wire-Pod container currently does not publish an `armv7` image, so
32-bit ARM Home Assistant installations are not supported.

## Installation

1. In Home Assistant, open **Settings → Apps → App store**.
2. Open **⋮ → Repositories**.
3. Add `https://github.com/KaeShoru/wire-pod-addon-v2`.
4. Install **Wire-Pod for Vector**.
5. Start the app and select **Open Web UI**.
6. Complete Wire-Pod's own setup wizard.

The repository and its GHCR package must be public before repository-based
installation can work. During development, copy the `wire-pod` directory into
Home Assistant's `/local_apps` directory and reload the app store.

## Why this version is lightweight

- Wire-Pod is built once in GitHub Actions, not on the Home Assistant device.
- The upstream base image is pinned by digest for reproducible builds.
- There is no Python runtime, MQTT sidecar, nginx, or duplicate process manager.
- Wire-Pod stores persistent state directly in Home Assistant's `/data` volume.
- The app uses the host network because Vector needs ports 80, 443, 8080 and
  8084 plus local-network discovery.

See [app documentation](wire-pod/DOCS.md) for setup, ports, backups and
troubleshooting.

## Upstream

- [Wire-Pod source and Docker image](https://github.com/kercre123/wire-pod)
- [WirePod packaged releases](https://github.com/kercre123/WirePod/releases)
- [Home Assistant app documentation](https://developers.home-assistant.io/docs/apps/)

This project is an independent packaging of Wire-Pod and is not affiliated with
Anki, Digital Dream Labs, Wire-Pod, or Home Assistant.
