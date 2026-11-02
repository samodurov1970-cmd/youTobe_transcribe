from sentence_transformers import CrossEncoder

print("Loading reranker...")
model = CrossEncoder("BAAI/bge-reranker-v2-m3", max_length=512)

pairs = [
    ("как правильно ставить стоп-лосс",
     "Remember, we enter on the bullish engulfing, so the stop loss would be below down here. The very conservative stop loss is below the swing low."),
    ("как правильно ставить стоп-лосс",
     "Now a $5 stock has 20% halt band. That's $1 share. It's gonna halt at $6."),
    ("как правильно ставить стоп-лосс",
     "The forecast is a hike from 3.75% to 4%, with eight members expected to vote for the hike."),
    ("как правильно ставить стоп-лосс",
     "Set your stop before the number, and if you're not sure, just watch the first five minutes."),
]

print("Scoring...")
scores = model.predict(pairs)
for (q, c), s in zip(pairs, scores):
    print(f"  score={s:+.4f}  | {c[:80]}...")

    