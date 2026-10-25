package engine

import (
	"errors"
	"sync"
	"time"

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
	mu            sync.Mutex
	host          int
	id            string
	gameState     *domain.GameState
	roomState     RoomState
	turnTimer     *time.Timer
	turnSeq       uint64
	handleTimeout func(roomID string)
	onRoomEnded   func(roomID string)
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

func (r *Room) SetOnRoomEnded(fn func(string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onRoomEnded = fn
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
	r.resetTurnTimer()
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

func (r *Room) RemovePlayer(playerID int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_ = r.gameState.PlayerLeft(playerID)

	// If room was in waiting lobby and host left, transfer host to next player
	if r.roomState == RoomStateWaiting && r.host == playerID {
		newHost := r.gameState.FirstPlayerID()
		if newHost != -1 {
			r.host = newHost
		}
	}

	// If game is now over
	if r.gameState.IsGameOver() {
		r.handleGameOver()
		return
	}

	// If still playing, reset the turn timer for the new turn player
	if r.roomState == RoomStatePlaying {
		r.resetTurnTimer()
	}
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
		r.handleGameOver()
		return nil
	}
	r.resetTurnTimer()
	return nil
}

func (r *Room) PassTurn(playerId int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.roomState != RoomStatePlaying {
		return domain.ErrGameNotStarted
	}
	err := r.gameState.PassTurn(playerId)
	if err != nil {
		return err
	}
	r.resetTurnTimer()
	return nil
}

func (r *Room) SayUno(playerID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.roomState != RoomStatePlaying {
		return domain.ErrGameNotStarted
	}
	return r.gameState.SayUno(playerID)
}

func (r *Room) CatchUno(accuserPlayerID int, targetPlayerID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.roomState != RoomStatePlaying {
		return domain.ErrGameNotStarted
	}
	return r.gameState.CatchUno(accuserPlayerID, targetPlayerID)
}

func (r *Room) DrawCard(playerId int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.roomState != RoomStatePlaying {
		return domain.ErrGameNotStarted
	}
	err := r.gameState.DrawCard(playerId)
	if err != nil {
		return err
	}
	if r.gameState.IsGameOver() {
		r.roomState = RoomStateEnd
		if r.turnTimer != nil {
			r.turnTimer.Stop()
		}
		return nil
	}
	r.resetTurnTimer()
	return nil
}

func (r *Room) DebugSnapshot() *domain.GetGameSnapShot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gameState.DebugGameSnapshot()
}

func (r *Room) GetSnapshot(playerID int) *domain.GetGameSnapShot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gameState.GetGameSnapShot(playerID)
}

func (r *Room) SetNotifier(fn func(string)) {
	r.handleTimeout = fn
}

func (r *Room) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roomState = RoomStateEnd
	if r.turnTimer != nil {
		r.turnTimer.Stop()
		r.turnTimer = nil
	}
}

func (r *Room) resetTurnTimer() {
	if r.turnTimer != nil {
		r.turnTimer.Stop()
	}
	if r.roomState != RoomStatePlaying {
		return
	}
	r.turnSeq++
	currentSeq := r.turnSeq
	r.turnTimer = time.AfterFunc(62500*time.Millisecond, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		// Ignore stale timer (if player made a move right at the buzzer)
		if r.turnSeq != currentSeq || r.roomState != RoomStatePlaying {
			return
		}
		// Auto-move: Draw 1 card for the AFK player (if not already drawn) and pass
		turnPlayerID := r.gameState.CurrentTurnPlayerID()
		if !r.gameState.HasDrawnCard() {
			_ = r.gameState.DrawCard(turnPlayerID)
		}
		_ = r.gameState.PassTurn(turnPlayerID)
		if r.gameState.IsGameOver() {
			r.roomState = RoomStateEnd
			if r.handleTimeout != nil {
				go r.handleTimeout(r.id)
			}
			return
		}
		// Trigger broadcast so all connected players see the move immediately!
		if r.handleTimeout != nil {
			go r.handleTimeout(r.id)
		}
		// Reset timer for the next player!
		r.resetTurnTimer()
	})
}

func (r *Room) handleGameOver() {
	// make it idempotenet
	// One player leaves
	// after, say 2 seconds another player leaves

	if r.roomState == RoomStateEnd {
		return
	}
	r.roomState = RoomStateEnd
	if r.turnTimer != nil {
		r.turnTimer.Stop()
		r.turnTimer = nil
	}
	time.AfterFunc(30*time.Second, func() {
		r.mu.Lock()
		fn := r.onRoomEnded
		roomID := r.id
		r.mu.Unlock()

		if fn != nil {
			fn(roomID)
		}
	})
}
