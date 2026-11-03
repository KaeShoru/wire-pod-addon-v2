# Vector OpenVoice prototype

CPU-only ARM64 tone-colour conversion prototype for Orange Pi 5B. It converts
Russian speech produced by the existing Silero service toward a locally uploaded
Vector reference using OpenVoice V2. The reference is never included in the
image and persists only in the Docker volume.

This is an experiment, not yet part of the wire-pod runtime path.

## Start

```bash
docker compose up -d
```

Upload a clean Vector reference once:

```bash
curl -f http://127.0.0.1:8623/reference \
  -F audio=@vector-reference.wav
```

Convert an existing Silero WAV:

```bash
curl -f http://127.0.0.1:8623/convert \
  -F audio=@aidar-natural.wav \
  -F tau=0.3 \
  -o vector-openvoice.wav
```

Endpoints: `GET /health`, `POST /reference`, `POST /convert`.

OpenVoice V2 is MIT licensed. The image pins upstream commit
`74a1d147b17a8c3092dd5430504bd83ef6c7eb23` and the official
`myshell-ai/OpenVoiceV2` converter checkpoint. Source TTS audio remains subject
to the licence of its TTS model.
