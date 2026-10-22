import json, sys, glob, os

for path in sorted(glob.glob(r"C:\Users\user\GO\youTobe_transcribe\backup\*.json")):
    try:
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
        print(f"{os.path.basename(path)}  ->  {data.get('name', '<no name>')}")
    except Exception as e:
        print(f"{os.path.basename(path)}  ->  ERROR: {e}")
