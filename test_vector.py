#!/usr/bin/env python3
import sys
from sentence_transformers import SentenceTransformer
import json

if len(sys.argv) < 2:
    print("usage: python test_vector.py \"вопрос\"")
    sys.exit(1)

question = sys.argv[1]
model = SentenceTransformer("intfloat/multilingual-e5-base")
emb = model.encode(["query: " + question], normalize_embeddings=True)[0]
print(json.dumps([float(x) for x in emb]))