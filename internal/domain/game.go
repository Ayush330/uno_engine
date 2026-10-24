package domain

import (
	"errors"
	"fmt"
	"slices"
)

type Direction int8
type StateName uint8

var (
	ErrGameNotStarted        = errors.New("domain: game has not started yet")
	ErrNotEnoughPlayers      = errors.New("domain: at least 2 players required to start")
	ErrGameAlreadyStarted    = errors.New("domain: game has already started")
	ErrCannotDistributeCards = errors.New("domain: cannot distribute cards")
	ErrInvalidTurn           = errors.New("domain: invalid player turn")
	ErrInvalidCardPlayed     = errors.New("domain: Invalid card played")
	ErrPlayerNotFound        = errors.New("domain: Player not found")
	ErrNextPlayerNotFound    = errors.New("domain: Next Player Turn Not Found")
)

const (
	MIN_PLAYERS_IN_GAME = 2
	MAX_PLAYERS_IN_GAME = 4
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
	lastAction       string
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
		lastAction:       "Waiting for players to join...",
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
	g.lastAction = fmt.Sprintf("Game started! 7 cards dealt to all players. %s's turn.", g.players[0].name)
	return nil
}

func (g *GameState) PlayCard(playerId int, card Card, color Color) error {
	// color change only work when the card is KindWild or KindWildDrawFour
	// First we have to validate if it is player turn
	currentPlayer := g.players[g.currentTurnIndex]
	if playerId != currentPlayer.id {
		return ErrInvalidTurn
	}
	// validate if it is a correct card
	if !card.IsValid() {
		return ErrInvalidCardPlayed
	}

	player := currentPlayer
	if !slices.Contains(player.handCards, card) {
		return ErrInvalidCardPlayed
	}
	err := g.discardPile.UpdateState(card, color)
	if err != nil {
		return err
	}
	err = player.RemoveCardFromHand(card)
	if err != nil {
		return err
	}

	// Record action message for player display
	switch card.Kind() {
	case KindWild:
		g.lastAction = fmt.Sprintf("%s played Wild! Active color set to %s.", currentPlayer.name, color)
	case KindWildDrawFour:
		g.lastAction = fmt.Sprintf("%s played Wild Draw 4! Active color is %s.", currentPlayer.name, color)
	case KindDrawTwo:
		g.lastAction = fmt.Sprintf("%s played %s Draw 2!", currentPlayer.name, card.Color())
	case KindSkip:
		g.lastAction = fmt.Sprintf("%s played %s Skip!", currentPlayer.name, card.Color())
	case KindReverse:
		g.lastAction = fmt.Sprintf("%s played %s Reverse!", currentPlayer.name, card.Color())
	default:
		g.lastAction = fmt.Sprintf("%s played %s %d.", currentPlayer.name, card.Color(), card.Value())
	}

	// remove the player from game if it has no cards
	if len(player.handCards) == 0 {
		g.lastAction = fmt.Sprintf("🏆 %s has 0 cards and won the game!", currentPlayer.name)
		currentPlayer.UpdateFinishedPlaying(true)
	}
	var countOfActivePlayers int
	for _, player := range g.players {
		if !player.FinishedPlaying() {
			countOfActivePlayers++
		}
	}
	if countOfActivePlayers == 1 {
		g.stateName = StateGameOver
		return nil
	}
	g.setNextTurnIndex(card.Kind())
	return nil
}

func (g *GameState) setNextTurnIndex(kind Kind) {
	switch kind {
	case KindSkip:
		nextTurn := findNextPlayerIndex(g.currentTurnIndex, g.direction, g.players)
		nextTurn = findNextPlayerIndex(nextTurn, g.direction, g.players)
		g.currentTurnIndex = nextTurn
	case KindReverse:
		g.direction = -1 * g.direction
		nextTurn := findNextPlayerIndex(g.currentTurnIndex, g.direction, g.players)
		g.currentTurnIndex = nextTurn
	case KindDrawTwo, KindWildDrawFour:
		victimPlayerIndex := findNextPlayerIndex(g.currentTurnIndex, g.direction, g.players)
		// pick 4/2 cards pile
		var numOfCardsToDraw uint8
		if kind == KindDrawTwo {
			numOfCardsToDraw = 2
		} else {
			numOfCardsToDraw = 4
		}
		cards, err := g.drawpile.DrawCards(numOfCardsToDraw)
		if err != nil {
			panic(err)
		}
		// add it to currentplayers hand
		victimPlayer := g.players[victimPlayerIndex]
		victimPlayer.AddToHand(cards)
		nextTurn := findNextPlayerIndex(victimPlayerIndex, g.direction, g.players)
		g.currentTurnIndex = nextTurn
	default:
		nextTurn := findNextPlayerIndex(g.currentTurnIndex, g.direction, g.players)
		g.currentTurnIndex = nextTurn
	}
}

func findNextPlayerIndex(curr int, direction Direction, p []*Player) int {
	// find length of players
	lenPlayers := len(p)
	// for next player, we do
	// first suppose dir is +ve
	// if currenty index is 2.
	// next index should be 3, great
	// but if current is 3 next should be 0
	// so we are using modulo operator
	// but what if we do uno reverse now
	// if currIndex is 2
	// next is -1 , cool
	// but what if it is 0
	// next is -1
	//but it should be len(players)
	NextPlayerIndexRaw := func(currentIndex int) int {
		nextIndex := (currentIndex + int(direction) + lenPlayers) % lenPlayers
		if nextIndex < 0 {
			nextIndex = lenPlayers - 1
		}
		return nextIndex
	}
	for i := 0; i < lenPlayers; i++ {
		nextIndex := NextPlayerIndexRaw(curr)
		player := p[nextIndex]
		if !player.FinishedPlaying() {
			return nextIndex
		}
		curr = nextIndex
	}
	return -1
}

func (g *GameState) DrawCard(playerID int) error {
	//Verify player turn
	// is playing
	// have cards
	if g.players[g.currentTurnIndex].id != playerID {
		return ErrInvalidTurn
	}
	player := g.players[g.currentTurnIndex]
	if player.FinishedPlaying() {
		return ErrInvalidTurn
	}
	card, err := g.drawpile.DrawCards(1)
	if err != nil {
		return err
	}
	// if it can be played immediately play it
	// else add it to the players hand card..
	// We need to determine the type of card that is drawn
	// color
	var color Color
	switch card[0].Kind() {
	case KindWild, KindWildDrawFour:
		// need to ask the user
		// for now we are continuing with the active color of the game
		color = g.discardPile.ActiveColor()
	default:
		color = card[0].Color()
	}
	player.AddCardToHand(card[0])
	err = g.PlayCard(playerID, card[0], color)
	if err != nil {
		// Card was NOT played: just pass turn to next player normally!
		g.lastAction = fmt.Sprintf("%s drew a card and passed.", player.name)
		g.currentTurnIndex = findNextPlayerIndex(g.currentTurnIndex, g.direction, g.players)
	}
	return nil
}

func (g *GameState) IsGameOver() bool {
	return g.stateName == StateGameOver
}

type GetGameSnapShot struct {
	PlayerId     int          `json:"playerId"`
	TurnPlayerID int          `json:"turnPlayerId"`
	ActiveColor  Color        `json:"activeColor"`
	Direction    Direction    `json:"direction"`
	DiscardPile  Card         `json:"discardPile"`
	DrawPile     int          `json:"drawPile"`
	Players      []PlayerInfo `json:"players"`
	LastAction   string       `json:"lastAction"`
}

type PlayerInfo struct {
	PlayerId      int    `json:"playerId"`
	PlayerName    string `json:"playerName"`
	IsActive      bool   `json:"isActive"`
	HandCardCount int    `json:"handCardCount"`
	HandCards     []Card `json:"handCards"`
}

func (g *GameState) GetGameSnapShot(playerID int) *GetGameSnapShot {
	pInfo := make([]PlayerInfo, len(g.players))
	for i := 0; i < len(g.players); i++ {
		playerHandCards := []Card{}
		if g.players[i].id == playerID {
			playerHandCards = g.players[i].handCards
		}
		info := PlayerInfo{
			PlayerId:      g.players[i].id,
			PlayerName:    g.players[i].name,
			IsActive:      !g.players[i].FinishedPlaying(),
			HandCardCount: len(g.players[i].handCards),
			HandCards:     playerHandCards,
		}
		pInfo[i] = info
	}
	turnPlayerID := -1
	if g.currentTurnIndex != -1 {
		turnPlayerID = g.players[g.currentTurnIndex].id
	}
	return &GetGameSnapShot{
		PlayerId:     playerID,
		TurnPlayerID: turnPlayerID,
		ActiveColor:  g.discardPile.ActiveColor(),
		Direction:    g.direction,
		DiscardPile:  g.discardPile.cards[len(g.discardPile.cards)-1],
		DrawPile:     len(g.drawpile.cards),
		Players:      pInfo,
		LastAction:   g.lastAction,
	}
}
