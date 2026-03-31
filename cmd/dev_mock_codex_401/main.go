package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19191", "listen address")
	flag.Parse()

	handler := http.NewServeMux()
	handler.HandleFunc("/responses", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"type":    "invalid_api_key",
				"message": "demo local mock 401",
				"code":    "invalid_api_key",
			},
		})
	})
	handler.HandleFunc("/responses/compact", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"type":    "invalid_api_key",
				"message": "demo local mock 401",
				"code":    "invalid_api_key",
			},
		})
	})

	log.Printf("dev mock codex 401 listening on %s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}
