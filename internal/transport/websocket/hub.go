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

type Client struct {
	playerID int
	conn     *websocket.Conn
	mu       sync.Mutex
}

// Need this to prevenet the critical ops of writing to a same websocket conn from mulctiple goroutine. It will paninc.
// THis got introduced because of auto turn..
func (c *Client) SendJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

type Hub struct {
	roomManager *engine.RoomManager
	mu          sync.RWMutex
	// string is for room instance and then it have connections per player ID
	rooms map[string]map[int]*Client
}

func CreateNewHub(rm *engine.RoomManager) *Hub {
	return &Hub{
		roomManager: rm,
		mu:          sync.RWMutex{},
		rooms:       make(map[string]map[int]*Client),
	}
}

func (h *Hub) Register(roomID string, playerID int, conn *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.rooms[roomID]; !exists {
		h.rooms[roomID] = make(map[int]*Client)
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
	h.mu.RLock()
	playerConn, ok := h.rooms[roomID]
	if !ok {
		h.mu.RUnlock()
		return
	}
	type temp struct {
		playerID int
		conn     *Client
	}
	var copyTemp []temp
	for playerID, conn := range playerConn {
		copyTemp = append(copyTemp, temp{
			playerID: playerID,
			conn:     conn,
		})
	}
	h.mu.RUnlock()
	room, err := h.roomManager.GetRoom(roomID)
	if err != nil {
		return
	}
	for _, temp := range copyTemp {
		snapshot := room.GetSnapshot(temp.playerID)
		err := temp.conn.SendJSON(snapshot)
		if err != nil {
			log.Printf("Error: %v", err)
		}
	}
}

type ClientAction struct {
	Action   string       `json:"action"`
	Card     domain.Card  `json:"card"`
	Color    domain.Color `json:"color"`
	TargetID int          `json:"targetId"`
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
	client := &Client{
		playerID: playerID,
		conn:     conn,
		mu:       sync.Mutex{},
	}
	h.Register(roomID, playerID, client)
	room.SetNotifier(h.BroadCast)
	h.BroadCast(roomID)
	go h.readPump(room, roomID, playerID, client)
}

func (h *Hub) readPump(room *engine.Room, roomID string, playerID int, conn *Client) {
	defer func() {
		h.Unregister(roomID, playerID)
		conn.conn.Close()
		h.BroadCast(roomID)
	}()

	for {
		var action ClientAction
		err := conn.conn.ReadJSON(&action)
		if err != nil {
			break
		}
		var actionErr error
		switch action.Action {
		case "START_GAME":
			actionErr = room.StartGame(playerID)
		case "PLAY_CARD":
			actionErr = room.PlayCard(playerID, action.Card, action.Color)
		case "DRAW_CARD":
			actionErr = room.DrawCard(playerID)
		case "PASS_TURN":
			actionErr = room.PassTurn(playerID)
		case "SAY_UNO":
			actionErr = room.SayUno(playerID)
		case "CATCH_UNO":
			actionErr = room.CatchUno(playerID, action.TargetID)
		}

		if actionErr != nil {
			_ = conn.SendJSON(map[string]any{
				"type":    "ERROR",
				"message": actionErr.Error(),
			})
			continue
		}

		h.BroadCast(roomID)
	}
}
