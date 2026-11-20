# Embedding sidecar. Kept separate from backend/build/Dockerfile.service
# because it is the only Python runtime in the repo; the Go image stays
# scratch-thin and this image owns the ONNX runtime.
FROM python:3.12-slim

# FASTEMBED_CACHE_PATH pins the download target so the model lands in a known
# layer, not in the temp dir fastembed would default to (a restart would then
# re-download). OMP_NUM_THREADS=1 matches the one-thread session in app.py.
ENV FASTEMBED_CACHE_PATH=/models \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    OMP_NUM_THREADS=1

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

COPY docker/embedder/requirements.txt .
RUN pip install -r requirements.txt

# Download weights at build time. A first request must never wait on the
# download, and an offline host must still be able to serve.
RUN python -c "from fastembed import TextEmbedding; TextEmbedding(model_name='sentence-transformers/all-MiniLM-L6-v2')"

COPY docker/embedder/app.py docker/embedder/selfcheck.py .

# The sidecar's contract with the Go client (embed.Dimension) is "one
# unit-length 384-dim vector per text". Assert it here so a wrong model name or
# a post-processing change fails the build instead of quietly degrading search
# to keyword-only at runtime. EMBEDDING_MODEL is the bare sentence-transformers
# id the compose files pass, so this also proves app.py normalizes it to the hub
# name fastembed expects.
RUN EMBEDDING_MODEL=all-MiniLM-L6-v2 python selfcheck.py

EXPOSE 8096

HEALTHCHECK --interval=10s --timeout=5s --start-period=40s --retries=5 \
    CMD curl -fsS http://localhost:8096/healthz || exit 1

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8096"]
