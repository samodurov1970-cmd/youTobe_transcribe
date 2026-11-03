import sys
import os
import glob
import subprocess
import tempfile
import json

os.environ["HF_HUB_DISABLE_PROGRESS_BARS"] = "1"
os.environ["HF_HUB_DISABLE_TELEMETRY"] = "1"

import builtins
if not getattr(builtins, "_patched_for_whisper", False):
    try:
        import av
        import faster_whisper.audio as fwa
        _ao = av.open
        def av_open(f, *a, **kw):
            kw.pop("metadata_errors", None)
            return _ao(f, *a, **kw)
        _oo = builtins.open
        def _open(f, *a, **kw):
            kw.pop("metadata_errors", None)
            return _oo(f, *a, **kw)
        av.open = av_open
        fwa.open = _open
        builtins.open = _open
        builtins._patched_for_whisper = True
    except Exception:
        pass

WHISPER_MODEL   = os.environ.get("WHISPER_MODEL", "medium")
WHISPER_DEVICE  = os.environ.get("WHISPER_DEVICE", "cpu")
WHISPER_COMPUTE = os.environ.get("WHISPER_COMPUTE", "int8")
CHUNK_SECONDS   = int(os.environ.get("WHISPER_CHUNK_SECONDS", "1800"))

WAV_OUT = os.path.join(tempfile.gettempdir(), "tmp16k.wav")


def to_wav(src: str) -> str:
    subprocess.run(
        ["ffmpeg", "-y", "-v", "error", "-i", src,
         "-ac", "1", "-ar", "16000", "-vn", WAV_OUT],
        check=True, timeout=7200
    )
    return WAV_OUT


def split_wav(wav_path: str, chunk_sec: int = CHUNK_SECONDS):
    out_dir = os.path.dirname(wav_path) or "."
    pattern = os.path.join(out_dir, "chunk_%03d.wav")
    for old in glob.glob(os.path.join(out_dir, "chunk_*.wav")):
        try:
            os.remove(old)
        except OSError:
            pass
    subprocess.run(
        ["ffmpeg", "-y", "-v", "error", "-i", wav_path,
         "-f", "segment", "-segment_time", str(chunk_sec),
         "-ac", "1", "-ar", "16000", "-vn", pattern],
        check=True, timeout=7200
    )
    return sorted(glob.glob(os.path.join(out_dir, "chunk_*.wav")))


def main():
    if len(sys.argv) < 2:
        print(json.dumps({"error": "no audio path"}), file=sys.stderr)
        sys.exit(1)

    audio_path = sys.argv[1]
    if not os.path.exists(audio_path):
        print(json.dumps({"error": f"file not found: {audio_path}"}), file=sys.stderr)
        sys.exit(1)

    try:
        from faster_whisper import WhisperModel

        print(f"loading model={WHISPER_MODEL} device={WHISPER_DEVICE} compute={WHISPER_COMPUTE}", file=sys.stderr)
        model = WhisperModel(WHISPER_MODEL, device=WHISPER_DEVICE, compute_type=WHISPER_COMPUTE)

        wav = to_wav(audio_path)
        chunks = split_wav(wav, CHUNK_SECONDS)
        if not chunks:
            chunks = [wav]

        print(f"chunks={len(chunks)}", file=sys.stderr)

        parts = []
        for j, ch in enumerate(chunks, 1):
            print(f"chunk {j}/{len(chunks)}: {os.path.basename(ch)}", file=sys.stderr)
            try:
                segs, info = model.transcribe(
                    ch, beam_size=5, vad_filter=True, condition_on_previous_text=False,
                )
                txt = " ".join(s.text.strip() for s in segs).strip()
                if txt:
                    parts.append(txt)
                print(f"chunk {j}/{len(chunks)} ok len={len(txt)} lang={getattr(info, 'language', '?')}", file=sys.stderr)
            except Exception as e:
                print(f"chunk {j} ERROR: {e}", file=sys.stderr)
            finally:
                if ch != wav:
                    try:
                        os.remove(ch)
                    except OSError:
                        pass

        text = " ".join(parts).strip()
        try:
            os.remove(wav)
        except OSError:
            pass
        print(text)
    except Exception as e:
        print(f"FATAL: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()