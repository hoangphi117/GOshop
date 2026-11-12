"""Text embedding sidecar for the recommendations service.

all-MiniLM-L6-v2, CPU only. It runs in its own container because a torch
process needs several hundred MB resident, which does not fit the 160m cap the
Go services share.

The model is baked into the image at build time: a container that downloads
weights on first request would stall the first shopper after every restart.
"""

import logging
import os
from typing import List

import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from sentence_transformers import SentenceTransformer

MODEL_NAME = os.environ.get("EMBEDDING_MODEL", "all-MiniLM-L6-v2")
# Matches vector(384) in the recommendations migration; a change here needs a
# migration and a re-embedding pass.
DIMENSION = 384
# MiniLM truncates to 256 tokens anyway; capping input keeps a pathological
# product description from costing a full batch of padding.
MAX_CHARS = 2000

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("embedder")

app = FastAPI(title="GOshop embedder")
model: SentenceTransformer | None = None


def get_model() -> SentenceTransformer:
    global model
    if model is None:
        log.info("loading %s", MODEL_NAME)
        # One thread: the container has a single CPU allowance and torch's
        # default pool oversubscribes it, which is slower, not faster.
        torch.set_num_threads(int(os.environ.get("EMBEDDING_THREADS", "1")))
        model = SentenceTransformer(MODEL_NAME, device="cpu")
        log.info("model ready, dim=%d", model.get_sentence_embedding_dimension())
    return model


class EmbedRequest(BaseModel):
    texts: List[str] = Field(..., min_length=1, max_length=256)


class EmbedResponse(BaseModel):
    vectors: List[List[float]]
    model: str
    dimension: int


@app.get("/healthz")
def healthz() -> dict:
    return {"status": "ok", "model": MODEL_NAME, "loaded": model is not None}


@app.post("/embed", response_model=EmbedResponse)
def embed(req: EmbedRequest) -> EmbedResponse:
    texts = [t.strip()[:MAX_CHARS] for t in req.texts]
    if not any(texts):
        raise HTTPException(status_code=400, detail="texts must not be empty")

    m = get_model()
    # normalize_embeddings matches vector_cosine_ops: cosine on unit vectors is
    # a plain dot product, so the HNSW index stays meaningful.
    vectors = m.encode(
        texts,
        batch_size=16,
        normalize_embeddings=True,
        convert_to_numpy=True,
        show_progress_bar=False,
    )

    if vectors.shape[1] != DIMENSION:
        raise HTTPException(
            status_code=500,
            detail=f"model produced {vectors.shape[1]} dims, expected {DIMENSION}",
        )

    return EmbedResponse(
        vectors=[v.tolist() for v in vectors],
        model=MODEL_NAME,
        dimension=DIMENSION,
    )
