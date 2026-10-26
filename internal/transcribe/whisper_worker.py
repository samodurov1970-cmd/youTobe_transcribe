import sys
import os
import signal

# Отключаем предупреждения HuggingFace
os.environ["HF_HUB_DISABLE_PROGRESS_BARS"] = "1"
os.environ["HF_HUB_DISABLE_TELEMETRY"] = "1"

# Таймаут (30 минут на одно видео)
TIMEOUT_SECONDS = 1800

def timeout_handler(signum, frame):
    print("Transcription timed out", file=sys.stderr)
    sys.exit(1)

def main():
    if len(sys.argv) < 2:
        sys.exit(1)

    audio_path = sys.argv[1]
    signal.signal(signal.SIGALRM, timeout_handler)
    signal.alarm(TIMEOUT_SECONDS)

    try:
        from faster_whisper import WhisperModel
        
        # Загружаем модель (device="cpu" и int8 для работы в Docker без GPU)
        model = WhisperModel("medium", device="cpu", compute_type="int8")
        
        segments, info = model.transcribe(
            audio_path,
            beam_size=5,
            vad_filter=True,
            language="ru",
            condition_on_previous_text=False
        )
        
        text = " ".join([segment.text.strip() for segment in segments])
        signal.alarm(0)
        
        # Печатаем чистый текст (без JSON)
        print(text)
        
    except Exception as e:
        print(str(e), file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()