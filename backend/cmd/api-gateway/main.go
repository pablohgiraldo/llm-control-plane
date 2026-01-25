package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// TODO: wire pipeline stages (auth -> prompt -> policy -> router -> audit/metrics -> provider)
func main() {
	addr := envOrDefault("HTTP_ADDR", ":8080")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api-gateway listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		dur := time.Since(start)
		// minimal structured line
		log.Println(fmt.Sprintf(`{"method":"%s","path":"%s","duration_ms":%d}`, r.Method, r.URL.Path, dur.Milliseconds()))
	})
}

