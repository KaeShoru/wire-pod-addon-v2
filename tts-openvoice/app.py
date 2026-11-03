from __future__ import annotations

import logging
import os
import subprocess
import tempfile
import threading
import time
from contextlib import asynccontextmanager
from pathlib import Path

import torch
from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import Response
from openvoice.api import ToneColorConverter


logging.basicConfig(level=logging.INFO)
LOG = logging.getLogger("vector_openvoice")
CONFIG = Path(os.getenv("OPENVOICE_CONFIG", "/opt/openvoice-model/config.json"))
CHECKPOINT = Path(os.getenv("OPENVOICE_CHECKPOINT", "/opt/openvoice-model/checkpoint.pth"))
DATA = Path(os.getenv("OPENVOICE_DATA", "/data"))
REFERENCE = DATA / "vector-reference.wav"
MAX_AUDIO_BYTES = 25 * 1024 * 1024

_converter: ToneColorConverter | None = None
_target_se: torch.Tensor | None = None
_lock = threading.Lock()
_loaded_at = 0.0


def _decode_audio(source: Path, output: Path) -> None:
    result = subprocess.run(
        (
            "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(source),
            "-vn", "-ac", "1", "-ar", "22050", "-c:a", "pcm_s16le", str(output),
        ),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=90,
        check=False,
    )
    if result.returncode != 0:
        raise ValueError(result.stderr.decode("utf-8", errors="replace").strip())


def _embedding(path: Path) -> torch.Tensor:
    assert _converter is not None
    with torch.inference_mode():
        return _converter.extract_se(str(path))


def _load() -> None:
    global _converter, _target_se, _loaded_at
    torch.set_num_threads(max(1, int(os.getenv("TORCH_THREADS", "4"))))
    started = time.monotonic()
    _converter = ToneColorConverter(str(CONFIG), device="cpu", enable_watermark=False)
    _converter.load_ckpt(str(CHECKPOINT))
    DATA.mkdir(parents=True, exist_ok=True)
    if REFERENCE.exists():
        _target_se = _embedding(REFERENCE)
    _loaded_at = time.time()
    LOG.info("OpenVoice ready in %.2fs; reference=%s", time.monotonic() - started, _target_se is not None)


@asynccontextmanager
async def lifespan(_: FastAPI):
    _load()
    yield


app = FastAPI(title="Vector OpenVoice converter", version="0.1.0", lifespan=lifespan)


@app.get("/health")
def health() -> dict[str, object]:
    return {
        "ready": _converter is not None,
        "reference_ready": _target_se is not None,
        "backend": "openvoice-v2-cpu",
        "loaded_at": _loaded_at,
    }


async def _read_upload(audio: UploadFile) -> bytes:
    data = await audio.read(MAX_AUDIO_BYTES + 1)
    if not data or len(data) > MAX_AUDIO_BYTES:
        raise HTTPException(status_code=413, detail="audio must be between 1 byte and 25 MiB")
    return data


@app.post("/reference")
async def set_reference(audio: UploadFile = File(...)) -> dict[str, object]:
    global _target_se
    data = await _read_upload(audio)
    with tempfile.TemporaryDirectory(prefix="openvoice-ref-") as tmp:
        source = Path(tmp) / "upload"
        decoded = Path(tmp) / "reference.wav"
        source.write_bytes(data)
        try:
            _decode_audio(source, decoded)
            with _lock:
                target = _embedding(decoded)
                decoded.replace(REFERENCE)
                _target_se = target
        except Exception as exc:
            LOG.exception("Reference extraction failed")
            raise HTTPException(status_code=400, detail=str(exc)) from exc
    return {"reference_ready": True, "bytes": len(data)}


@app.post("/convert")
async def convert(audio: UploadFile = File(...), tau: float = Form(0.3)) -> Response:
    if _converter is None:
        raise HTTPException(status_code=503, detail="converter is not ready")
    if _target_se is None:
        raise HTTPException(status_code=409, detail="upload a Vector reference first")
    if not 0.0 <= tau <= 1.0:
        raise HTTPException(status_code=400, detail="tau must be between 0 and 1")

    data = await _read_upload(audio)
    started = time.monotonic()
    with tempfile.TemporaryDirectory(prefix="openvoice-convert-") as tmp:
        source = Path(tmp) / "upload"
        decoded = Path(tmp) / "source.wav"
        output = Path(tmp) / "converted.wav"
        source.write_bytes(data)
        try:
            _decode_audio(source, decoded)
            with _lock, torch.inference_mode():
                source_se = _embedding(decoded)
                _converter.convert(
                    audio_src_path=str(decoded),
                    src_se=source_se,
                    tgt_se=_target_se,
                    output_path=str(output),
                    tau=tau,
                )
            result = output.read_bytes()
        except Exception as exc:
            LOG.exception("Voice conversion failed")
            raise HTTPException(status_code=500, detail=str(exc)) from exc
    LOG.info("Converted %.2fs audio in %.2fs", len(data) / 44100.0, time.monotonic() - started)
    return Response(result, media_type="audio/wav", headers={"X-OpenVoice-Tau": str(tau)})
