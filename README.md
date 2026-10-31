# Wire-Pod for Home Assistant

A lightweight Home Assistant app repository for Anki Vector. It contains:

- **Wire-Pod for Vector** — the local voice server, with Home Assistant
  Ingress and a sidebar panel.
- **Vector (Anki / Digital Dream Labs)** — a companion bridge that exposes the
  robot's sensors, events and commands to the Vector HACS integration.

Neither app compiles Wire-Pod, Go, or Vosk on the Home Assistant host. Release
installations download pre-built multi-architecture images. This keeps
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
5. Start the app and select **Open Web UI**. Enable **Show in sidebar** on the
   app page if Home Assistant did not enable it automatically.
6. Complete Wire-Pod's own setup wizard.

## Home Assistant sensors and control

For battery charge, charger state, activity, camera, speaker and commands:

1. Install **Vector (Anki / Digital Dream Labs)** from the same app store.
   Leave `wirepod_url` set to `auto` and start it.
2. In HACS, add
   `https://github.com/witold-gren/home-assistant-vector-integration` as a
   custom **Integration**, install **Vector**, and restart Home Assistant.
3. Open **Settings → Devices & services** and accept the discovered Vector
   integration.

The integration creates battery and voltage sensors; binary sensors for
connection, charging, charger, touch, pickup, cliff and sleep; buttons for
charger, sleep, dance, explore and other behaviors; plus camera, speaker,
events and movement controls. MQTT is not required.

The repository and its GHCR package must be public before repository-based
installation can work. During development, copy the `wire-pod` directory into
Home Assistant's `/local_apps` directory and reload the app store.

## Why this version is lightweight

- Wire-Pod is built once in GitHub Actions, not on the Home Assistant device.
- The upstream base image is pinned by digest for reproducible builds.
- The small Ingress proxy is the only extra process in the Wire-Pod app.
- The Vector bridge is optional and runs as its own lightweight app.
- Wire-Pod stores persistent state directly in Home Assistant's `/data` volume.
- The app uses the host network because Vector needs ports 80, 443, 8080 and
  8084 plus local-network discovery.

See [Wire-Pod documentation](wire-pod/DOCS.md) and
[Vector bridge documentation](vector/DOCS.md) for setup and troubleshooting.

## Upstream

- [Wire-Pod source and Docker image](https://github.com/kercre123/wire-pod)
- [WirePod packaged releases](https://github.com/kercre123/WirePod/releases)
- [Vector bridge](https://github.com/witold-gren/home-assistant-vector-addon)
- [Vector integration](https://github.com/witold-gren/home-assistant-vector-integration)
- [Home Assistant app documentation](https://developers.home-assistant.io/docs/apps/)

The companion bridge is based on Witold Greń's MIT-licensed
`home-assistant-vector-addon`; its license and copyright notice are included in
the `vector` directory.

This project is an independent packaging of Wire-Pod and is not affiliated with
Anki, Digital Dream Labs, Wire-Pod, or Home Assistant.
