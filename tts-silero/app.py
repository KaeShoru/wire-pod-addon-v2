from __future__ import annotations

import io
import logging
import os
import subprocess
import threading
import time
import wave
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Literal

import torch
from fastapi import FastAPI, HTTPException
from fastapi.responses import Response
from pydantic import BaseModel, Field


LOG = logging.getLogger("vector_tts")
MODEL_PATH = Path(os.getenv("SILERO_MODEL_PATH", "/opt/silero/v5_6_ru.pt"))
SAMPLE_RATE = 24_000
SPEAKERS = {"aidar", "baya", "kseniya", "xenia", "eugene"}
PROFILES = {"natural", "vector", "vector_bright"}

_model = None
_model_lock = threading.Lock()
_loaded_at = 0.0


class TTSRequest(BaseModel):
    text: str = Field(min_length=1, max_length=1_000)
    speaker: str = "aidar"
    profile: Literal["natural", "vector", "vector_bright"] = "vector"
    speed: float = Field(default=0.92, ge=0.75, le=1.20)


def _load_model() -> None:
    global _model, _loaded_at
    started = time.monotonic()
    threads = max(1, int(os.getenv("TORCH_THREADS", "4")))
    torch.set_num_threads(threads)
    LOG.info("Loading Silero model %s with %d CPU threads", MODEL_PATH, threads)
    _model = torch.package.PackageImporter(str(MODEL_PATH)).load_pickle(
        "tts_models", "model"
    )
    _model.to(torch.device("cpu"))
    _loaded_at = time.time()
    LOG.info("Silero model ready in %.2fs", time.monotonic() - started)


def _tensor_to_wav(audio: torch.Tensor) -> bytes:
    pcm = (
        audio.detach()
        .to(device="cpu", dtype=torch.float32)
        .flatten()
        .clamp(-1.0, 1.0)
        .mul(32767.0)
        .to(dtype=torch.int16)
        .numpy()
        .tobytes()
    )
    output = io.BytesIO()
    with wave.open(output, "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(SAMPLE_RATE)
        wav.writeframes(pcm)
    return output.getvalue()


def _filter_for(profile: str, speed: float) -> str:
    if profile == "natural":
        return f"atempo={speed:.5f},alimiter=limit=0.92"

    # Vector sounds small and bright, but its fundamental pitch is not unusually
    # high. Shift the formants upward while restoring the fundamental with
    # Rubber Band, then add only a little synthetic texture.
    formant_ratio = 1.18 if profile == "vector" else 1.27
    restore = 1.0 / formant_ratio
    crush_mix = 0.055 if profile == "vector" else 0.10
    bits = 13 if profile == "vector" else 11
    tempo = speed * (0.94 if profile == "vector" else 0.91)
    raised_rate = int(SAMPLE_RATE * formant_ratio)
    return ",".join(
        (
            f"asetrate={raised_rate}",
            f"aresample={SAMPLE_RATE}",
            (
                "rubberband="
                f"pitch={restore:.7f}:tempo={restore:.7f}:"
                "formant=preserved:transients=smooth:detector=soft"
            ),
            f"atempo={tempo:.5f}",
            "highpass=f=150",
            "lowpass=f=7600",
            f"acrusher=bits={bits}:mix={crush_mix:.3f}:aa=0.75",
            "acompressor=threshold=-18dB:ratio=2.6:attack=5:release=90:makeup=1.5",
            "alimiter=limit=0.90",
        )
    )


def _apply_profile(wav: bytes, profile: str, speed: float) -> bytes:
    command = (
        "ffmpeg",
        "-hide_banner",
        "-loglevel",
        "error",
        "-i",
        "pipe:0",
        "-af",
        _filter_for(profile, speed),
        "-ac",
        "1",
        "-ar",
        str(SAMPLE_RATE),
        "-c:a",
        "pcm_s16le",
        "-f",
        "wav",
        "pipe:1",
    )
    result = subprocess.run(
        command,
        input=wav,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
        timeout=90,
    )
    if result.returncode != 0:
        raise RuntimeError(result.stderr.decode("utf-8", errors="replace").strip())
    return result.stdout


@asynccontextmanager
async def lifespan(_: FastAPI):
    _load_model()
    yield


app = FastAPI(title="Vector Silero TTS", version="0.1.0", lifespan=lifespan)


@app.get("/health")
def health() -> dict[str, object]:
    return {
        "ready": _model is not None,
        "model": "silero-v5_6_ru",
        "sample_rate": SAMPLE_RATE,
        "loaded_at": _loaded_at,
    }


@app.get("/voices")
def voices() -> dict[str, object]:
    return {"speakers": sorted(SPEAKERS), "profiles": sorted(PROFILES)}


@app.post("/tts")
def tts(request: TTSRequest) -> Response:
    if _model is None:
        raise HTTPException(status_code=503, detail="model is not ready")
    if request.speaker not in SPEAKERS:
        raise HTTPException(status_code=400, detail="unknown speaker")

    started = time.monotonic()
    try:
        with _model_lock, torch.inference_mode():
            audio = _model.apply_tts(
                text=request.text.strip(),
                speaker=request.speaker,
                sample_rate=SAMPLE_RATE,
            )
        wav = _apply_profile(_tensor_to_wav(audio), request.profile, request.speed)
    except Exception as exc:
        LOG.exception("Synthesis failed")
        raise HTTPException(status_code=500, detail=str(exc)) from exc

    elapsed = time.monotonic() - started
    LOG.info(
        "Synthesized %d chars speaker=%s profile=%s speed=%.2f in %.2fs",
        len(request.text),
        request.speaker,
        request.profile,
        request.speed,
        elapsed,
    )
    return Response(
        content=wav,
        media_type="audio/wav",
        headers={"X-Synthesis-Seconds": f"{elapsed:.3f}"},
    )
