# Changelog

## 2.3.0

- Added optional local Russian TTS through a configurable HTTP endpoint.
- Added a lightweight 20-band vocoder based on Vector's original
  `Krotos_Vocoder_12` soundbank settings: 180 Hz carrier, 5 ms envelopes and a
  -2 dB output ceiling.
- Kept the built-in phonetic voice as an automatic fallback when the external
  TTS service is disabled or unavailable.
- The Home Assistant host only performs inexpensive DSP; the TTS model remains
  on a separate machine such as an Orange Pi.

## 2.2.0

- Added automatic Russian-to-Latin phonetic conversion before native Vector speech.
- Kept Vector's built-in voice; no external TTS or Home Assistant relay is required.
- Made OpenAI-compatible streaming tolerant of usage-only chunks and replies without final punctuation.

## 2.1.0

- Added Home Assistant Ingress with a Wire-Pod entry for the sidebar.
- Added a small authenticated reverse proxy that adapts Wire-Pod's absolute
  web paths to the Home Assistant Ingress path.
- Added the companion Vector bridge app to this repository for battery and
  state sensors, events, camera, speaker and robot commands.

## 2.0.2

- Made the Wire-Pod server the container's managed foreground process so Home
  Assistant can track and stop it reliably.
- Enabled Docker's lightweight init process for signal forwarding and zombie
  process cleanup.

## 2.0.1

- Fixed the Web UI URL template so Home Assistant Supervisor accepts and lists
  the app in the store.
- Promoted the app from experimental to the default stable stage.

## 2.0.0

- Rebuilt the repository using the supported Home Assistant app layout.
- Replaced the non-starting Python/bashio container with the official Wire-Pod
  multi-architecture image.
- Pinned the upstream image by digest for reproducible releases.
- Removed local Wire-Pod, Vosk and Go compilation from installation.
- Persisted all upstream Wire-Pod state in `/data`.
- Added host networking, health monitoring and cold-backup metadata.
- Limited supported architectures to the upstream-tested `aarch64` and `amd64`.
