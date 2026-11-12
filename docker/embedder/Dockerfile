# Embedding sidecar. Kept separate from backend/build/Dockerfile.service
# because it is the only Python runtime in the repo; the Go image stays
# scratch-thin and this image owns the torch dependency.
FROM python:3.12-slim

# HF_HOME pins the download target so the model lands in the layer we copy
# between stages, not in a random cache dir.
ENV HF_HOME=/models \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    OMP_NUM_THREADS=1

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

COPY docker/embedder/requirements.txt .
# CPU-only torch first: sentence-transformers would otherwise resolve the CUDA
# build, adding ~2.5 GB of unusable libraries to the image.
RUN pip install --extra-index-url https://download.pytorch.org/whl/cpu torch \
    && pip install -r requirements.txt

# Download weights at build time. A first request must never wait on the HF
# download, and an offline host must still be able to serve.
RUN python -c "from sentence_transformers import SentenceTransformer; \
    SentenceTransformer('all-MiniLM-L6-v2')"

COPY docker/embedder/app.py .

EXPOSE 8096

HEALTHCHECK --interval=10s --timeout=5s --start-period=40s --retries=5 \
    CMD curl -fsS http://localhost:8096/healthz || exit 1

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8096"]
