"""Text embedding sidecar for the recommendations service.

all-MiniLM-L6-v2, CPU only, via ONNX Runtime (fastembed). It keeps its own
container because the model and its runtime want a couple hundred MB resident,
which does not fit the 160m cap the Go services share.

This ran on torch until the VPS needed it: the model is 22M parameters, so the
~700 MB torch runtime was almost all overhead on a 1.9 GB host. The ONNX runtime
is the same 384-dim model, so the Go contract (embed.Dimension) and the
vector(384) column are unchanged.

The model is baked into the image at build time: a container that downloads
weights on first request would stall the first shopper after every restart.
"""

import logging
import os
from typing import List

import numpy as np
from fastembed import TextEmbedding
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

# fastembed keys on the full hub name ("sentence-transformers/all-MiniLM-L6-v2");
# the bare "all-MiniLM-L6-v2" that sentence-transformers accepted is NOT in its
# registry. Accept both, so a compose file still passing the bare id does not
# fail the sidecar at the first request.
_RAW_MODEL = os.environ.get("EMBEDDING_MODEL", "sentence-transformers/all-MiniLM-L6-v2")
MODEL_NAME = _RAW_MODEL if "/" in _RAW_MODEL else f"sentence-transformers/{_RAW_MODEL}"
# Matches vector(384) in the recommendations migration; a change here needs a
# migration and a re-embedding pass.
DIMENSION = 384
# MiniLM truncates to 256 tokens anyway; capping input keeps a pathological
# product description from costing a full batch of padding.
MAX_CHARS = 2000

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("embedder")

app = FastAPI(title="GOshop embedder")
model: TextEmbedding | None = None


def get_model() -> TextEmbedding:
    global model
    if model is None:
        log.info("loading %s", MODEL_NAME)
        # One thread: the container has a single CPU allowance and onnxruntime's
        # default pool oversubscribes it, which is slower, not faster.
        model = TextEmbedding(
            model_name=MODEL_NAME,
            threads=int(os.environ.get("EMBEDDING_THREADS", "1")),
        )
        log.info("model ready, dim=%d", DIMENSION)
    return model


def embed_texts(m: TextEmbedding, texts: List[str]) -> List[List[float]]:
    """One unit-length vector per text, in the same order.

    Unit length is what vector_cosine_ops assumes: cosine on unit vectors is a
    plain dot product, so the HNSW index stays meaningful. fastembed mean-pools
    already; normalizing here makes the contract explicit rather than a default
    of the model's post-processing.
    """
    vectors: List[List[float]] = []
    for raw in m.embed(texts):
        v = np.asarray(raw, dtype=np.float32)
        norm = float(np.linalg.norm(v))
        vectors.append((v / norm if norm > 0 else v).tolist())
    return vectors


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

    vectors = embed_texts(get_model(), texts)

    # A partial batch is an error: silently dropping a text would shift every
    # later index, and a wrong width would make pgvector reject the write anyway.
    if len(vectors) != len(texts):
        raise HTTPException(
            status_code=500,
            detail=f"model produced {len(vectors)} vectors, expected {len(texts)}",
        )
    if vectors and len(vectors[0]) != DIMENSION:
        raise HTTPException(
            status_code=500,
            detail=f"model produced {len(vectors[0])} dims, expected {DIMENSION}",
        )

    return EmbedResponse(vectors=vectors, model=MODEL_NAME, dimension=DIMENSION)
