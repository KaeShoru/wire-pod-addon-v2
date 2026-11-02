# Vector TTS for RK3588

Local Russian Piper TTS service for Orange Pi 5B and other RK3588 devices.
The dynamic encoder runs through ONNX Runtime on CPU; the flow and HiFi-GAN
decoder run on the Rockchip NPU through RKNN 2.3.2.

The image contains the `ru_RU-dmitri-medium` voice converted specifically for
RK3588. It does not replace the host's `librknnrt.so` and can coexist with an
RKLLM service that uses its own `librkllmrt.so`.

## Start

```bash
docker compose up -d
```

The API listens on port 8621:

```bash
curl -f http://127.0.0.1:8621/health
curl -f http://127.0.0.1:8621/tts \
  -H 'Content-Type: application/json' \
  --data '{"text":"Привет! Я Вектор.","language":"ru_RU","speed":1.0}' \
  -o vector-test.wav
```

## Hardware access

RK3588 exposes its NPU through a DRM render node, commonly
`/dev/dri/renderD129`. The Compose file passes `/dev/dri` to the container but
does not run it as privileged.

## Model provenance

- Voice: `rhasspy/piper-voices` `ru_RU/dmitri/medium` (repository MIT;
  model card states the dataset is CC0).
- Converter and API: `suharvest/rkvoice-stream` (Apache-2.0).
- Runtime: Rockchip RKNN Toolkit2 2.3.2.
