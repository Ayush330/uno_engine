package main

import (
	"log"
	"net/http"

	"github.com/Ayush330/uno_engine/internal/engine"
	customhttp "github.com/Ayush330/uno_engine/internal/transport/http"
	customws "github.com/Ayush330/uno_engine/internal/transport/websocket"
)

func main() {
	mux := http.NewServeMux()
	roomManager := engine.NewRoomManager()
	roomHandler := customhttp.CreateNewRoomHandler(roomManager)
	roomHandler.RegisterRoutes(mux)

	hub := customws.CreateNewHub(roomManager)
	mux.HandleFunc("/ws", hub.ServeWs)

	log.Println("UNO Engine server started on http://localhost:8080")
	if err := http.ListenAndServe(":8080", enableCORS(mux)); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
