"""Build-time check for the embedder sidecar.

The Go client (embed.Dimension) and the vector(384) column both assume one
unit-length 384-dim vector per text. Run by the Dockerfile right after the
weights are downloaded, so a wrong model name or a post-processing change fails
the build rather than silently degrading search to keyword-only at runtime.

Runs the exact production path (embed_texts + get_model), not a re-implementation.
"""

import numpy as np

from app import DIMENSION, embed_texts, get_model

vectors = embed_texts(get_model(), ["a red camera", "wireless headphones"])

assert len(vectors) == 2, f"expected 2 vectors, got {len(vectors)}"
for v in vectors:
    assert len(v) == DIMENSION, f"expected {DIMENSION} dims, got {len(v)}"
    norm = float(np.linalg.norm(v))
    assert abs(norm - 1.0) < 1e-3, f"expected a unit vector, got norm {norm}"

print(f"ok: 2 texts -> 2 unit-length {DIMENSION}-dim vectors")
