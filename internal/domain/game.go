package domain

import "errors"

type Direction int8
type StateName uint8

var (
	ErrNotEnoughPlayers      = errors.New("domain: at least 2 players required to start")
	ErrGameAlreadyStarted    = errors.New("domain: game has already started")
	ErrCannotDistributeCards = errors.New("domain: cannot distribute cards")
)

const (
	MIN_PLAYERS_IN_GAME = 2
	COUNT_OF_HAND_CARDS = 7
)

const (
	CLOCKWISE        Direction = 1
	COUNTERCLOCKWISE Direction = -1
)

const (
	StateWaitingForPlayers StateName = iota
	StateDistributingCards
	StateWaitingForTurn
	StateGameOver
)

type GameState struct {
	id               int
	drawpile         *DrawPile
	discardPile      *DiscardPile
	players          []*Player
	direction        Direction
	currentTurnIndex int
	stateName        StateName
}

func NewGameState(id int) *GameState {
	deck := NewDeck()
	if err := deck.Shuffle(); err != nil {
		panic(err)
	}
	// Add The first Cards from the deck to the discard pile
	pickedCard := deck[len(deck)-1]
	deck = deck[:len(deck)-1]
	roughCards := []Card{}
	for pickedCard.Kind() != KindNumber {
		roughCards = append(roughCards, pickedCard)
		pickedCard = deck[len(deck)-1]
		deck = deck[:len(deck)-1]
	}
	deck = append(deck, roughCards...)
	discardPile, err := NewDiscardPile(pickedCard)
	if err != nil {
		panic(err)
	}
	drawPile, err := NewDrawPile(deck)
	if err != nil {
		panic(err)
	}
	return &GameState{
		id:               id,
		drawpile:         drawPile,
		discardPile:      discardPile,
		players:          nil,
		direction:        CLOCKWISE,
		currentTurnIndex: -1,
		stateName:        StateWaitingForPlayers,
	}
}

// Add a Player
func (g *GameState) AddPlayer(name string, id int) error {
	if g.stateName != StateWaitingForPlayers {
		return errors.New("error: cannot add player, game has already started")
	}
	playerRec := &Player{
		name: name,
		id:   id,
	}
	g.players = append(g.players, playerRec)
	return nil
}

func (g *GameState) GameStart() error {
	if len(g.players) < MIN_PLAYERS_IN_GAME {
		return ErrNotEnoughPlayers
	}
	if g.stateName != StateWaitingForPlayers {
		return ErrGameAlreadyStarted
	}
	g.stateName = StateDistributingCards
	// distribute cards
	var err error
	var handCards []Card
	for _, player := range g.players {
		handCards, err = g.drawpile.DrawCards(COUNT_OF_HAND_CARDS)
		if err != nil {
			return ErrCannotDistributeCards
		}
		player.AddToHand(handCards)
	}
	g.stateName = StateWaitingForTurn
	g.currentTurnIndex = 0
	return nil
}
