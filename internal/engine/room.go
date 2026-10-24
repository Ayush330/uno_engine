package domain

import (
	"errors"
	"sync"

	"github.com/Ayush330/uno_engine/internal/domain/utility"
)

type RoomState uint8

var (
	ErrOnlyHostCanStartTheGame = errors.New("domain: Only host can start the game.")
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
	gameState *GameState
	roomState RoomState
}

func CreateNewRoom(hostName string, hostId int) *Room {
	gameState := NewGameState(utility.GenerateIntInBetweenLimits(1, 10000))
	gameState.AddPlayer(hostName, hostId)
	return &Room{
		host:      hostId,
		id:        utility.GenerateRoomId(),
		gameState: gameState,
		roomState: RoomStateWaiting,
	}
}

func (r *Room) StartGame(requestorId int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Only Host can start the game
	if r.host != requestorId {
		return ErrOnlyHostCanStartTheGame
	}
	// the root state should be Waiting
	if r.roomState != RoomStateWaiting {
		return ErrGameAlreadyStarted
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
		return ErrGameAlreadyStarted
	}
	return r.gameState.AddPlayer(name, id)
}

func (r *Room) PlayCard(requestorId int, card Card, color Color) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.roomState != RoomStatePlaying {
		return ErrGameNotStarted
	}
	err := r.gameState.PlayCard(requestorId, card, color)
	if err != nil {
		return err
	}
	if r.gameState.stateName == StateGameOver {
		r.roomState = RoomStateEnd
	}
	return nil
}

func (r *Room) DrawCard(playerId int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.roomState != RoomStatePlaying {
		return ErrGameNotStarted
	}
	return r.gameState.DrawCard(playerId)
}
