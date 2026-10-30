# Changelog

## 2.0.0

- Rebuilt the repository using the supported Home Assistant app layout.
- Replaced the non-starting Python/bashio container with the official Wire-Pod
  multi-architecture image.
- Pinned the upstream image by digest for reproducible releases.
- Removed local Wire-Pod, Vosk and Go compilation from installation.
- Persisted all upstream Wire-Pod state in `/data`.
- Added host networking, health monitoring and cold-backup metadata.
- Limited supported architectures to the upstream-tested `aarch64` and `amd64`.
