FROM python:3.11-slim-trixie

ARG SILERO_MODEL_URL=https://models.silero.ai/models/tts/ru/v5_6_ru.pt

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    SILERO_MODEL_PATH=/opt/silero/v5_6_ru.pt \
    TORCH_THREADS=4

<<<<<<< HEAD
# RHVoice lives in Debian non-free, while the slim python images enable only main.
RUN sed -i 's/^Components: main$/Components: main contrib non-free non-free-firmware/' \
        /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates curl espeak-ng ffmpeg rhvoice rhvoice-russian \
=======
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl espeak-ng ffmpeg \
>>>>>>> origin/main
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
        "fastapi==0.119.0" \
        "numpy==2.1.2" \
        "torch==2.8.0" \
        "uvicorn==0.37.0"

RUN mkdir -p /opt/silero \
    && curl --fail --location --retry 4 \
        "${SILERO_MODEL_URL}" \
        --output /opt/silero/v5_6_ru.pt \
    && test "$(stat -c %s /opt/silero/v5_6_ru.pt)" -eq 147664410

WORKDIR /app
COPY app.py /app/app.py

EXPOSE 8622

HEALTHCHECK --interval=30s --timeout=5s --start-period=45s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8622/health', timeout=3)" || exit 1

CMD ["python", "-m", "uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8622"]
