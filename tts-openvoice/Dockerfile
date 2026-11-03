FROM python:3.11-slim-bookworm

ARG OPENVOICE_COMMIT=74a1d147b17a8c3092dd5430504bd83ef6c7eb23
ARG OPENVOICE_MODEL_BASE=https://huggingface.co/myshell-ai/OpenVoiceV2/resolve/main/converter

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH=/opt/OpenVoice \
    OPENVOICE_CONFIG=/opt/openvoice-model/config.json \
    OPENVOICE_CHECKPOINT=/opt/openvoice-model/checkpoint.pth \
    OPENVOICE_DATA=/data \
    TORCH_THREADS=4

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl ffmpeg git \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
        "torch==2.8.0" \
        "numpy==2.1.2" \
        "librosa==0.11.0" \
        "soundfile==0.13.1" \
        "inflect==7.5.0" \
        "eng-to-ipa==0.0.2" \
        "unidecode==1.3.8" \
        "pypinyin==0.53.0" \
        "cn2an==0.5.23" \
        "jieba==0.42.1" \
        "fastapi==0.119.0" \
        "python-multipart==0.0.20" \
        "uvicorn==0.37.0"

RUN git clone --filter=blob:none --no-checkout https://github.com/myshell-ai/OpenVoice.git /opt/OpenVoice \
    && git -C /opt/OpenVoice checkout "${OPENVOICE_COMMIT}" \
    && rm -rf /opt/OpenVoice/.git \
    && sed -i \
        -e "s/super().__init__(\*args, \*\*kwargs)/enable_watermark = kwargs.pop('enable_watermark', True)\\n        super().__init__(*args, **kwargs)/" \
        -e "s/if kwargs.get('enable_watermark', True):/if enable_watermark:/" \
        /opt/OpenVoice/openvoice/api.py \
    && mkdir -p /opt/openvoice-model \
    && curl --fail --location --retry 4 "${OPENVOICE_MODEL_BASE}/config.json" -o /opt/openvoice-model/config.json \
    && curl --fail --location --retry 4 "${OPENVOICE_MODEL_BASE}/checkpoint.pth" -o /opt/openvoice-model/checkpoint.pth \
    && test "$(stat -c %s /opt/openvoice-model/checkpoint.pth)" -eq 131320490

WORKDIR /app
COPY app.py /app/app.py

VOLUME ["/data"]
EXPOSE 8623

HEALTHCHECK --interval=30s --timeout=5s --start-period=90s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8623/health', timeout=3)" || exit 1

CMD ["python", "-m", "uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8623"]
