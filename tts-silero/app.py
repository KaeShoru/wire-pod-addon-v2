from __future__ import annotations

import io
import logging
import os
import shutil
import subprocess
import tempfile
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
PROFILES = {"natural", "vocoder_source", "vector", "vector_bright"}
ENGINES = {"silero", "espeak", "rhvoice"}
# Classic eSpeak voice variants. The klatt family is the most "1980s robot"
# sounding; m*/f* shift the register; croak/whisper add vocal texture.
ESPEAK_VARIANTS = {
    "",
    "m1", "m2", "m3", "m4", "m5", "m6", "m7",
    "f1", "f2", "f3", "f4", "f5",
    "klatt", "klatt2", "klatt3", "klatt4",
    "croak", "whisper", "whisperf",
}
ESPEAK_WPM_BASE = 175
ESPEAK_PITCH_BASE = 50
# RHVoice: statistical parametric synthesis — the classic intelligible
# "screen reader robot" voice. `variant` selects one of its Russian voices.
RHVOICE_VOICE_DIR = Path("/usr/share/RHVoice/voices")
RHVOICE_DEFAULT_VOICE = "aleksandr"

_model = None
_model_lock = threading.Lock()
_loaded_at = 0.0
_espeak_ready = False
_rhvoice_ready = False
_rhvoice_voices: set[str] = set()


class TTSRequest(BaseModel):
    text: str = Field(min_length=1, max_length=1_000)
    engine: Literal["silero", "espeak", "rhvoice"] = "silero"
    speaker: str = "aidar"
    profile: Literal["natural", "vocoder_source", "vector", "vector_bright"] = (
        "natural"
    )
    speed: float = Field(default=0.92, ge=0.75, le=1.20)
    pitch: float = Field(default=1.0, ge=0.60, le=1.80)
    variant: str = ""


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


def _detect_espeak() -> bool:
    global _espeak_ready
    _espeak_ready = shutil.which("espeak-ng") is not None
    if _espeak_ready:
        LOG.info("eSpeak NG engine available (classic formant synthesis)")
    else:
        LOG.warning("espeak-ng binary not found; espeak engine disabled")
    return _espeak_ready


def _detect_rhvoice() -> bool:
    global _rhvoice_ready, _rhvoice_voices
    _rhvoice_voices = set()
    if RHVOICE_VOICE_DIR.is_dir():
        _rhvoice_voices = {
            entry.name.lower()
            for entry in RHVOICE_VOICE_DIR.iterdir()
            if entry.is_dir()
        }
    _rhvoice_ready = shutil.which("RHVoice-test") is not None and bool(
        _rhvoice_voices
    )
    if _rhvoice_ready:
        LOG.info("RHVoice engine available, voices: %s", sorted(_rhvoice_voices))
    else:
        LOG.warning("RHVoice-test binary or voices not found; rhvoice engine disabled")
    return _rhvoice_ready


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


def _filter_for(profile: str, speed: float, pitch: float) -> str:
    if profile == "natural":
        return f"atempo={speed:.5f},alimiter=limit=0.92"

    if profile == "vocoder_source":
        # Prepare a clean, pitch-matched source for the downstream vocoder.
        # Shifting formants with the fundamental is intentional here; keeping
        # male formants while raising only F0 still sounds like a pitched man.
        return ",".join(
            (
                (
                    "rubberband="
                    f"pitch={pitch:.7f}:tempo={speed:.7f}:"
                    "formant=shifted:transients=smooth:detector=soft"
                ),
                "highpass=f=120",
                "lowpass=f=7800",
                "alimiter=limit=0.90",
            )
        )

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


def _synth_espeak(text: str, variant: str, speed: float, pitch: float) -> bytes:
    """Classic formant synthesis via eSpeak NG.

    This is intentionally a robotic 2000s-style voice: it does not try to
    sound human, which makes it a far better source for Vector's vocoder
    character than a neural TTS. `speed` and `pitch` are multipliers mapped
    onto eSpeak's native words-per-minute and 0-99 pitch scales.
    """
    voice = "ru" + (f"+{variant}" if variant else "")
    wpm = max(80, min(370, round(ESPEAK_WPM_BASE * speed)))
    espeak_pitch = max(0, min(99, round(ESPEAK_PITCH_BASE * pitch)))
    command = (
        "espeak-ng",
        "-v",
        voice,
        "-s",
        str(wpm),
        "-p",
        str(espeak_pitch),
        "--stdout",
        text,
    )
    result = subprocess.run(
        command,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
        timeout=60,
    )
    if result.returncode != 0 or not result.stdout.startswith(b"RIFF"):
        detail = result.stderr.decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"espeak-ng failed ({result.returncode}): {detail}")
    return result.stdout


def _synth_rhvoice(text: str, voice: str, speed: float) -> bytes:
    """Statistical parametric synthesis via RHVoice.

    The voice is clearly synthetic yet stays smooth and intelligible — the
    classic Russian "screen reader robot" sound. Only `speed` is mapped to
    RHVoice's native rate; pitch shifting happens afterwards in ffmpeg,
    because the native -t option makes the HTS voice strain for high notes
    instead of cleanly shifting the whole voice.
    """
    rate = max(50, min(400, round(100 * speed)))
    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
        out_path = tmp.name
    try:
        command = (
            "RHVoice-test",
            "-p",
            voice,
            "-R",
            str(SAMPLE_RATE),
            "-r",
            str(rate),
            "-o",
            out_path,
        )
        result = subprocess.run(
            command,
            input=text.encode("utf-8"),
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            check=False,
            timeout=60,
        )
        if result.returncode != 0:
            detail = result.stderr.decode("utf-8", errors="replace").strip()
            raise RuntimeError(f"RHVoice failed ({result.returncode}): {detail}")
        wav = Path(out_path).read_bytes()
        if not wav.startswith(b"RIFF"):
            raise RuntimeError("RHVoice produced no WAV output")
        return wav
    finally:
        Path(out_path).unlink(missing_ok=True)


def _filter_rhvoice(pitch: float) -> str:
    """Post-processing for RHVoice voices.

    Pitch is shifted with Rubber Band (formants move with the fundamental,
    giving a smaller robot rather than a straining voice) and the level is
    boosted ahead of the limiter, because RHVoice renders quietly.
    """
    chain = []
    if abs(pitch - 1.0) > 1e-3:
        chain.append(
            "rubberband="
            f"pitch={pitch:.7f}:tempo=1.0:"
            "formant=shifted:transients=smooth:detector=soft"
        )
    chain.extend(("highpass=f=120", "lowpass=f=7800", "volume=1.6", "alimiter=limit=0.90"))
    return ",".join(chain)


def _run_filter(wav: bytes, filter_chain: str) -> bytes:
    command = (
        "ffmpeg",
        "-hide_banner",
        "-loglevel",
        "error",
        "-i",
        "pipe:0",
        "-af",
        filter_chain,
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


def _apply_profile(wav: bytes, profile: str, speed: float, pitch: float) -> bytes:
    return _run_filter(wav, _filter_for(profile, speed, pitch))


@asynccontextmanager
async def lifespan(_: FastAPI):
    _load_model()
    _detect_espeak()
    _detect_rhvoice()
    yield


app = FastAPI(title="Vector Silero TTS", version="0.4.2", lifespan=lifespan)


@app.get("/health")
def health() -> dict[str, object]:
    return {
        "ready": _model is not None,
        "model": "silero-v5_6_ru",
        "sample_rate": SAMPLE_RATE,
        "loaded_at": _loaded_at,
        "engines": {
            "silero": _model is not None,
            "espeak": _espeak_ready,
            "rhvoice": _rhvoice_ready,
        },
    }


@app.get("/voices")
def voices() -> dict[str, object]:
    return {
        "engines": sorted(ENGINES),
        "speakers": sorted(SPEAKERS),
        "profiles": sorted(PROFILES),
        "espeak_variants": sorted(ESPEAK_VARIANTS),
        "rhvoice_voices": sorted(_rhvoice_voices),
        "rhvoice_default_voice": RHVOICE_DEFAULT_VOICE,
    }


@app.post("/tts")
def tts(request: TTSRequest) -> Response:
    started = time.monotonic()
    try:
        if request.engine == "espeak":
            if not _espeak_ready:
                raise HTTPException(
                    status_code=503, detail="espeak-ng engine is not available"
                )
            if request.variant not in ESPEAK_VARIANTS:
                raise HTTPException(
                    status_code=400,
                    detail=f"unknown espeak variant {request.variant!r}",
                )
            # eSpeak already sounds robotic; apply only loudness normalisation,
            # not the Silero-specific timbre profiles.
            raw = _synth_espeak(
                request.text.strip(), request.variant, request.speed, request.pitch
            )
            wav = _apply_profile(raw, "natural", 1.0, 1.0)
        elif request.engine == "rhvoice":
            if not _rhvoice_ready:
                raise HTTPException(
                    status_code=503, detail="RHVoice engine is not available"
                )
            voice = request.variant.strip().lower() or RHVOICE_DEFAULT_VOICE
            if voice not in _rhvoice_voices:
                raise HTTPException(
                    status_code=400,
                    detail=f"unknown RHVoice voice {voice!r}; "
                    f"available: {sorted(_rhvoice_voices)}",
                )
            # RHVoice pitch is shifted afterwards with Rubber Band; the native
            # -t option strains the HTS voice instead of raising it cleanly.
            raw = _synth_rhvoice(request.text.strip(), voice, request.speed)
            wav = _run_filter(raw, _filter_rhvoice(request.pitch))
        else:
            if _model is None:
                raise HTTPException(status_code=503, detail="model is not ready")
            if request.speaker not in SPEAKERS:
                raise HTTPException(status_code=400, detail="unknown speaker")
            with _model_lock, torch.inference_mode():
                audio = _model.apply_tts(
                    text=request.text.strip(),
                    speaker=request.speaker,
                    sample_rate=SAMPLE_RATE,
                )
            wav = _apply_profile(
                _tensor_to_wav(audio), request.profile, request.speed, request.pitch
            )
    except HTTPException:
        raise
    except Exception as exc:
        LOG.exception("Synthesis failed")
        raise HTTPException(status_code=500, detail=str(exc)) from exc

    elapsed = time.monotonic() - started
    LOG.info(
        "Synthesized %d chars engine=%s speaker=%s variant=%s profile=%s "
        "speed=%.2f pitch=%.2f in %.2fs",
        len(request.text),
        request.engine,
        request.speaker,
        request.variant,
        request.profile,
        request.speed,
        request.pitch,
        elapsed,
    )
    return Response(
        content=wav,
        media_type="audio/wav",
        headers={"X-Synthesis-Seconds": f"{elapsed:.3f}"},
    )
