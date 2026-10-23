package main

import (
	"net/http"
	"os"
	"time"

	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
	"youTobe_transcribe/internal/web"

	_ "github.com/lib/pq"
)

func main() {
	logger.Init()
	log := logger.Log.With("component", "webui")

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Error("webui.no_dsn", "описание", "DATABASE_URL не задан")
		os.Exit(1)
	}
	if err := storage.Init(dsn); err != nil {
		log.Error("webui.db_init_failed", "error", err.Error())
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/trash", web.TrashPage(log))
	mux.HandleFunc("/trash/action", web.TrashSetAction(log))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/trash", http.StatusSeeOther)
	})

	addr := os.Getenv("WEBUI_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Info("webui.start", "addr", addr, "описание", "UI запущен")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("webui.serve_failed", "error", err.Error())
		os.Exit(1)
	}
}