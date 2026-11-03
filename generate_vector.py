#!/usr/bin/env python3
"""Generate embedding vector for search quality test."""
from sentence_transformers import SentenceTransformer
import json

m = SentenceTransformer('intfloat/multilingual-e5-base')
e = m.encode(['query: что такое ликвидность в трейдинге?'], normalize_embeddings=True)[0]
print('VECTOR_START', json.dumps([float(x) for x in e]), 'VECTOR_END')