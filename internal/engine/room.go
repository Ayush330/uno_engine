package engine

import (
	"errors"
	"sync"

	"github.com/Ayush330/uno_engine/internal/domain"
	"github.com/Ayush330/uno_engine/internal/domain/utility"
)

type RoomState uint8

var (
	ErrOnlyHostCanStartTheGame = errors.New("engine: Only host can start the game.")
)

const (
	RoomStateWaiting RoomState = iota
	RoomStatePlaying
	RoomStateEnd
)

type Room struct {
	mu        sync.Mutex
	host      int
	id        string
	gameState *domain.GameState
	roomState RoomState
}

func CreateNewRoom(hostName string, hostId int) *Room {
	gameState := domain.NewGameState(utility.GenerateIntInBetweenLimits(1, 10000))
	gameState.AddPlayer(hostName, hostId)
	return &Room{
		host:      hostId,
		id:        utility.GenerateRoomId(),
		gameState: gameState,
		roomState: RoomStateWaiting,
	}
}

func (r *Room) ID() string {
	return r.id
}

func (r *Room) Host() int {
	return r.host
}

func (r *Room) State() RoomState {
	return r.roomState
}

func (r *Room) GameState() *domain.GameState {
	return r.gameState
}

func (r *Room) StartGame(requestorId int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Only Host can start the game
	if r.host != requestorId {
		return ErrOnlyHostCanStartTheGame
	}
	// the room state should be Waiting
	if r.roomState != RoomStateWaiting {
		return domain.ErrGameAlreadyStarted
	}
	// start the game state
	err := r.gameState.GameStart()
	if err != nil {
		return err
	}
	r.roomState = RoomStatePlaying
	return nil
}

func (r *Room) AddPlayer(name string, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.roomState != RoomStateWaiting {
		return domain.ErrGameAlreadyStarted
	}
	return r.gameState.AddPlayer(name, id)
}

func (r *Room) PlayCard(requestorId int, card domain.Card, color domain.Color) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.roomState != RoomStatePlaying {
		return domain.ErrGameNotStarted
	}
	err := r.gameState.PlayCard(requestorId, card, color)
	if err != nil {
		return err
	}
	if r.gameState.IsGameOver() {
		r.roomState = RoomStateEnd
	}
	return nil
}

func (r *Room) DrawCard(playerId int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.roomState != RoomStatePlaying {
		return domain.ErrGameNotStarted
	}
	return r.gameState.DrawCard(playerId)
}

func (r *Room) DebugSnapshot() *domain.GetGameSnapShot {
    r.mu.Lock()
    defer r.mu.Unlock()
    return r.gameState.DebugGameSnapshot()
}
