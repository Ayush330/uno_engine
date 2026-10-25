package http

import (
	"encoding/json"
	"net/http"

	"github.com/Ayush330/uno_engine/internal/engine"
)

type RoomHandler struct {
	roomManager *engine.RoomManager
}

func CreateNewRoomHandler(rm *engine.RoomManager) *RoomHandler {
	return &RoomHandler{
		roomManager: rm,
	}
}

type CreateRoomRequest struct {
	HostName string `json:"hostName"`
	HostID   int    `json:"hostId"`
}

type CreateRoomResponse struct {
	RoomID string `json:"roomId"`
	HostID int    `json:"hostId"`
}

func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	var req CreateRoomRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Failed to decode room creation request: "+err.Error(), http.StatusBadRequest)
		return
	}
	room, err := h.roomManager.AddRoom(req.HostName, req.HostID)
	if err != nil {
		http.Error(w, "Failed to create room: "+err.Error(), http.StatusBadRequest)
		return
	}
	response := CreateRoomResponse{
		RoomID: room.ID(),
		HostID: room.Host(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)

}

type JoinRoomRequest struct {
	PlayerName string `json:"playerName"`
	PlayerID   int    `json:"playerId"`
}

type JoinRoomResponse struct {
	RoomID   string `json:"roomId"`
	PlayerID int    `json:"playerId"`
}

func (h *RoomHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	// find the room
	room, err := h.roomManager.GetRoom(roomID)
	if err != nil {
		http.Error(w, "Room Not Found: "+err.Error(), http.StatusNotFound)
		return
	}
	var req JoinRoomRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Failed to decode join room request: "+err.Error(), http.StatusBadRequest)
		return
	}
	// make the player join the room
	err = room.AddPlayer(req.PlayerName, req.PlayerID)
	if err != nil {
		http.Error(w, "Cannot Join "+err.Error(), http.StatusBadRequest)
		return
	}
	response := JoinRoomResponse{
		RoomID:   room.ID(),
		PlayerID: req.PlayerID,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *RoomHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/rooms", h.CreateRoom)
	mux.HandleFunc("POST /api/rooms/{id}/join", h.JoinRoom)
	mux.HandleFunc("GET /api/rooms/debug/snapshots", h.GetRoomDetails)
}

func (h *RoomHandler) GetRoomDetails(w http.ResponseWriter, r *http.Request) {
	rm := h.roomManager.GetAllRooms()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rm)
}
