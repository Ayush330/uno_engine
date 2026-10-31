package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ayush330/uno_engine/internal/engine"
	"github.com/Ayush330/uno_engine/internal/telemetry"
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


	// otel
	otelCfg := telemetry.Config{
		GetActiveGames:   roomManager.RoomCount,
		GetActivePlayers: roomManager.TotalPlayerCount,
	}
	metricsHandler, shutdownOtel, err := telemetry.Setup(otelCfg)
	if err != nil {
		log.Fatalf("failed to set up telemetry: %v", err)
	}
	mux.Handle("/metrics", metricsHandler)

	// 3. Static Web Client
	// Using AI for this.....
	fs := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fs)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:    ":" + port,
		Handler: enableCORS(mux),
	}
	go func() {
		log.Printf("UNO Engine server started on http://localhost:%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	if err := shutdownOtel(ctx); err != nil {
		log.Printf("Error shutting down telemetry: %v", err)
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
