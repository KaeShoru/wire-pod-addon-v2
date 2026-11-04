# Vector Silero TTS prototype

CPU-only Russian TTS prototype for choosing a base voice and Vector-like audio
profile before integrating it into wire-pod. The image embeds Silero
`v5_6_ru`, so the Orange Pi does not need access to GitHub or the model host.

## Start

```bash
docker compose up -d
```

The service listens on port `8622` and exposes:

- `GET /health`
- `GET /voices`
- `POST /tts`

Example:

```bash
curl -f http://127.0.0.1:8622/tts \
  -H 'Content-Type: application/json' \
  --data '{"text":"Привет! Я Вектор.","speaker":"aidar","profile":"vocoder_source","speed":0.92,"pitch":1.10}' \
  -o vector-aidar.wav
```

Engines (`engine` field, default `silero`):

- `silero` — neural Russian voices. Speakers: `aidar`, `baya`, `kseniya`,
  `xenia`, `eugene`.
- `rhvoice` — RHVoice statistical parametric synthesis: clearly synthetic
  but smooth and intelligible, the classic Russian screen-reader robot
  sound. `variant` picks the voice (`aleksandr` by default; also `anna`,
  `elena`, `irina`, `artemiy`, `pavel`, `vitaliy-ng`, … — see `GET /voices`).
  `speed` and `pitch` map to RHVoice's native rate and pitch percentages.
- `espeak` — classic Klatt formant synthesis (the harsh "robot voice" of the
  2000s), Russian included. Optional `variant` selects an eSpeak NG voice
  variant: empty for the plain voice, `m1`–`m7`, `f1`–`f5`, `klatt`,
  `klatt2`, `klatt3`, `klatt4`, `croak`, `whisper`, `whisperf`.

For `espeak` and `rhvoice` the `speaker` field is ignored and profiles other
than `natural` are not applied.

```bash
curl -f http://127.0.0.1:8622/tts \
  -H 'Content-Type: application/json' \
  --data '{"text":"Привет! Я Вектор.","engine":"rhvoice","variant":"aleksandr","speed":0.9,"pitch":1.05}' \
  -o vector-rhvoice.wav

curl -f http://127.0.0.1:8622/tts \
  -H 'Content-Type: application/json' \
  --data '{"text":"Привет! Я Вектор.","engine":"espeak","variant":"klatt","speed":0.9,"pitch":1.1}' \
  -o vector-espeak-klatt.wav
```

Profiles:

- `natural` — uncoloured Silero output, with only tempo and peak limiting.
- `vocoder_source` — clean pitch and formant matching before Wire-Pod's
  vocoder; no bit crushing or compression. Use `pitch` to tune it.
- `vector` — moderate formant lift and subtle synthetic texture.
- `vector_bright` — a deliberately stronger comparison profile.

## Model and licence

Silero `v5_6_ru` is downloaded from the versioned URL documented by the
Silero project. Silero states that its non-CIS-base models use the repository's
CC-NC-BY licence. This prototype is intended for local, non-commercial use and
preserves attribution to the Silero team.
