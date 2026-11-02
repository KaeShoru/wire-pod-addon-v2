FROM python:3.11-slim-bookworm

ARG RKVOICE_STREAM_VERSION=0.2.0

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    LD_LIBRARY_PATH=/usr/lib \
    PYTHONUNBUFFERED=1 \
    OMP_NUM_THREADS=4 \
    TTS_BACKEND=piper_rknn \
    REQUIRE_TTS_BACKEND=1 \
    PIPER_MODEL_DIR=/opt/piper-models \
    PIPER_LANGUAGES=ru_RU \
    PIPER_DEFAULT_LANG=ru_RU \
    PIPER_MEL_LEN=256

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        espeak-ng \
        libgomp1 \
        libsndfile1 \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
        "numpy==1.26.4" \
        "onnxruntime==1.19.2" \
        "rknn-toolkit-lite2==2.3.2" \
        "rkvoice-stream==${RKVOICE_STREAM_VERSION}"

COPY librknnrt.so /usr/lib/librknnrt.so
COPY model/ /opt/piper-models/ru_RU/

RUN ldconfig \
    && python -c "assert b'librknnrt version: 2.3.2' in open('/usr/lib/librknnrt.so', 'rb').read()" \
    && cd /opt/piper-models/ru_RU \
    && sha256sum -c SHA256SUMS

EXPOSE 8621

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8621/health', timeout=3)" || exit 1

CMD ["python", "-m", "uvicorn", "rkvoice_stream.app.server:app", "--host", "0.0.0.0", "--port", "8621"]
