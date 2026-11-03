#!/usr/bin/env python3
"""Test vector search quality."""
import sys
sys.path.insert(0, '.')
from sentence_transformers import SentenceTransformer
import json

# Generate embedding for the query
m = SentenceTransformer('intfloat/multilingual-e5-base')
e = m.encode(['query: что такое ликвидность в трейдинге?'], normalize_embeddings=True)[0]

# Print the vector in the required format
vector_list = [float(x) for x in e]
vector_json = json.dumps(vector_list)
print('VECTOR_START')
print(vector_json)
print('VECTOR_END')