package websocket

import (
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/Ayush330/uno_engine/internal/domain"
	"github.com/Ayush330/uno_engine/internal/engine"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	roomManager *engine.RoomManager
	mu          sync.Mutex
	// string is for room instance and then it have connections per player ID
	rooms map[string]map[int]*websocket.Conn
}

func CreateNewHub(rm *engine.RoomManager) *Hub {
	return &Hub{
		roomManager: rm,
		mu:          sync.Mutex{},
		rooms:       make(map[string]map[int]*websocket.Conn),
	}
}

func (h *Hub) Register(roomID string, playerID int, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.rooms[roomID]; !exists {
		h.rooms[roomID] = make(map[int]*websocket.Conn)
	}
	h.rooms[roomID][playerID] = conn
}

func (h *Hub) Unregister(roomID string, playerID int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.rooms[roomID]; !exists {
		return
	}
	delete(h.rooms[roomID], playerID)
	if len(h.rooms[roomID]) == 0 {
		delete(h.rooms, roomID)
		h.roomManager.RemoveRoom(roomID)
	}
}

func (h *Hub) BroadCast(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[roomID]; !ok {
		return
	}
	room, err := h.roomManager.GetRoom(roomID)
	if err != nil {
		return
	}
	playerConn := h.rooms[roomID]
	for playerID, conn := range playerConn {
		snapshot := room.GameState().GetGameSnapShot(playerID)
		err := conn.WriteJSON(snapshot)
		if err != nil {
			log.Printf("Error: %v", err)
		}
	}
}

type ClientAction struct {
	Action string       `json:"action"`
	Card   domain.Card  `json:"card"`
	Color  domain.Color `json:"color"`
}

func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("roomId")
	playerIDStr := r.URL.Query().Get("playerId")
	playerID, err := strconv.Atoi(playerIDStr)
	if err != nil || roomID == "" {
		http.Error(w, "invalid room or player id", http.StatusBadRequest)
		return
	}

	room, err := h.roomManager.GetRoom(roomID)
	if err != nil {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("error upgrading: %v", err)
		return
	}
	h.Register(roomID, playerID, conn)
	h.BroadCast(roomID)
	go h.readPump(room, roomID, playerID, conn)
}

func (h *Hub) readPump(room *engine.Room, roomID string, playerID int, conn *websocket.Conn) {
	defer func() {
		h.Unregister(roomID, playerID)
		conn.Close()
		h.BroadCast(roomID)
	}()

	for {
		var action ClientAction
		err := conn.ReadJSON(&action)
		if err != nil {
			break
		}
		switch action.Action {
		case "START_GAME":
			_ = room.StartGame(playerID)
		case "PLAY_CARD":
			_ = room.PlayCard(playerID, action.Card, action.Color)
		case "DRAW_CARD":
			_ = room.DrawCard(playerID)

		}
		h.BroadCast(roomID)
	}
}
