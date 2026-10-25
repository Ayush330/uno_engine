package domain

import (
	"errors"
	"fmt"
	"slices"
	"time"
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
	id                 int
	drawpile           *DrawPile
	discardPile        *DiscardPile
	players            []*Player
	direction          Direction
	currentTurnIndex   int
	stateName          StateName
	lastAction         string
	turnDeadline       int64
	hasDrawnCard       bool
	vulnerablePlayerID int
	unoGraceDeadline   int64
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
		id:                 id,
		drawpile:           drawPile,
		discardPile:        discardPile,
		players:            nil,
		direction:          CLOCKWISE,
		currentTurnIndex:   -1,
		stateName:          StateWaitingForPlayers,
		lastAction:         "Waiting for players to join...",
		vulnerablePlayerID: -1,
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
	g.resetTurnDeadline()
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
	// Action taken: close any previous UNO vulnerability window
	if g.vulnerablePlayerID != playerId {
		g.vulnerablePlayerID = -1
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
		g.vulnerablePlayerID = -1
		g.unoGraceDeadline = 0
	} else if len(player.handCards) == 1 {
		// Only one card is left: start 2.5s grace period!
		player.SetSaidUno(false)
		g.vulnerablePlayerID = player.id
		g.unoGraceDeadline = time.Now().Add(2500 * time.Millisecond).UnixMilli()
	} else {
		// Hand has > 1 card: can never be vulnerable or have UNO
		player.SetSaidUno(false)
		g.vulnerablePlayerID = -1
		g.unoGraceDeadline = 0
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
	g.hasDrawnCard = false
	g.setNextTurnIndex(card.Kind())
	return nil
}

func (g *GameState) setNextTurnIndex(kind Kind) {
	g.resetTurnDeadline()
	switch kind {
	case KindSkip:
		nextTurn := findNextPlayerIndex(g.currentTurnIndex, g.direction, g.players)
		nextTurn = findNextPlayerIndex(nextTurn, g.direction, g.players)
		g.currentTurnIndex = nextTurn
	case KindReverse:
		g.direction = -1 * g.direction
		if g.countActivePlayers() == 2 {
			// In 2-player Uno, Reverse acts as Skip: the current player goes again!
			return
		}
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
		cards, _ := g.DrawCardWithReshuffle(numOfCardsToDraw)
		// add it to currentplayers hand
		victimPlayer := g.players[victimPlayerIndex]
		victimPlayer.AddToHand(cards)
		victimPlayer.SetSaidUno(false)
		if g.vulnerablePlayerID == victimPlayer.id {
			g.vulnerablePlayerID = -1
			g.unoGraceDeadline = 0
		}
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
	// Action taken: closes any active UNO vulnerability window
	g.vulnerablePlayerID = -1
	g.unoGraceDeadline = 0

	//Verify player turn
	// is playing
	// have cards
	if g.hasDrawnCard {
		return errors.New("domain: already drawn a card this turn")
	}
	if g.players[g.currentTurnIndex].id != playerID {
		return ErrInvalidTurn
	}
	player := g.players[g.currentTurnIndex]
	if player.FinishedPlaying() {
		return ErrInvalidTurn
	}
	card, err := g.DrawCardWithReshuffle(1)
	if err != nil {
		return err
	}
	// if it can be played immediately play it
	// else add it to the players hand card..
	// We need to determine the type of card that is drawn
	// color
	player.AddCardToHand(card[0])
	player.SetSaidUno(false)
	//removig force handling
	//err = g.PlayCard(playerID, card[0], color)
	g.hasDrawnCard = true
	g.lastAction = fmt.Sprintf("%s drew a card.", player.name)
	g.resetTurnDeadline()
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
	TurnDuration int          `json:"turnDuration"`
	TurnDeadline int64        `json:"turnDeadline"`
	HasDrawnCard bool         `json:"hasDrawnCard"`
}

type PlayerInfo struct {
	PlayerId          int    `json:"playerId"`
	PlayerName        string `json:"playerName"`
	IsActive          bool   `json:"isActive"`
	HandCardCount     int    `json:"handCardCount"`
	HandCards         []Card `json:"handCards"`
	HasSaidUno        bool   `json:"hasSaidUno"`
	IsVulnerableToUno bool   `json:"isVulnerableToUno"`
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
			HasSaidUno:    g.players[i].HasSaidUno(),
			IsVulnerableToUno: (g.players[i].id == g.vulnerablePlayerID &&
				len(g.players[i].handCards) == 1 &&
				!g.players[i].HasSaidUno() &&
				time.Now().UnixMilli() >= g.unoGraceDeadline),
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
		TurnDuration: 60,
		TurnDeadline: g.turnDeadline,
		HasDrawnCard: g.hasDrawnCard,
	}
}

func (g *GameState) DebugGameSnapshot() *GetGameSnapShot {
	pInfo := make([]PlayerInfo, len(g.players))
	for i := 0; i < len(g.players); i++ {
		playerHandCards := g.players[i].handCards
		info := PlayerInfo{
			PlayerId:      g.players[i].id,
			PlayerName:    g.players[i].name,
			IsActive:      !g.players[i].FinishedPlaying(),
			HandCardCount: len(g.players[i].handCards),
			HandCards:     playerHandCards,
			HasSaidUno:    g.players[i].HasSaidUno(),
			IsVulnerableToUno: (g.players[i].id == g.vulnerablePlayerID &&
				len(g.players[i].handCards) == 1 &&
				!g.players[i].HasSaidUno() &&
				time.Now().UnixMilli() >= g.unoGraceDeadline),
		}
		pInfo[i] = info
	}
	turnPlayerID := -1
	if g.currentTurnIndex != -1 {
		turnPlayerID = g.players[g.currentTurnIndex].id
	}
	var topDiscard Card
	if len(g.discardPile.cards) > 0 {
		topDiscard = g.discardPile.cards[len(g.discardPile.cards)-1]
	}
	return &GetGameSnapShot{
		PlayerId:     -1,
		TurnPlayerID: turnPlayerID,
		ActiveColor:  g.discardPile.ActiveColor(),
		Direction:    g.direction,
		DiscardPile:  topDiscard,
		DrawPile:     len(g.drawpile.cards),
		Players:      pInfo,
		LastAction:   g.lastAction,
		TurnDuration: 60,
		TurnDeadline: g.turnDeadline,
		HasDrawnCard: g.hasDrawnCard,
	}
}

func (g *GameState) DrawCardWithReshuffle(count uint8) ([]Card, error) {
	if len(g.drawpile.cards) < int(count) {
		newDeckPile, err := g.discardPile.CardsToUseWhenDrawPileIsEmpty()
		if err != nil {
			return nil, err
		}
		g.drawpile.cards = append(g.drawpile.cards, newDeckPile...)
		return g.drawpile.DrawCards(count)
	}
	return g.drawpile.DrawCards(count)
}

func (g *GameState) countActivePlayers() int {
	count := 0
	for _, p := range g.players {
		if !p.FinishedPlaying() {
			count++
		}
	}
	return count
}

func (g *GameState) CurrentTurnPlayerID() int {
	if g.currentTurnIndex == -1 {
		return -1
	}
	return g.players[g.currentTurnIndex].id
}

func (g *GameState) resetTurnDeadline() {
	g.turnDeadline = time.Now().Add(60 * time.Second).UnixMilli()
}

func (g *GameState) PassTurn(playerId int) error {
	if g.players[g.currentTurnIndex].id != playerId {
		return ErrInvalidTurn
	}
	if !g.hasDrawnCard {
		return errors.New("domain: has not drawn a card this turn")
	}
	g.hasDrawnCard = false
	g.vulnerablePlayerID = -1
	g.unoGraceDeadline = 0
	g.lastAction = fmt.Sprintf("%s passed the turn.", g.players[g.currentTurnIndex].name)
	g.currentTurnIndex = findNextPlayerIndex(g.currentTurnIndex, g.direction, g.players)
	g.resetTurnDeadline()
	return nil
}

func (g *GameState) HasDrawnCard() bool {
	return g.hasDrawnCard
}

func (g *GameState) SayUno(playerID int) error {
	player := g.findPlayerByID(playerID)
	if player == nil {
		return ErrPlayerNotFound
	}
	if player.FinishedPlaying() || len(player.handCards) != 1 {
		return errors.New("domain: can only shout UNO when holding exactly 1 card")
	}
	player.SetSaidUno(true)
	g.vulnerablePlayerID = -1
	g.unoGraceDeadline = 0
	g.lastAction = fmt.Sprintf("🔥 %s shouted UNO!", player.name)
	return nil
}

func (g *GameState) CatchUno(accuserPlayerID int, targetPlayerID int) error {
	accuser := g.findPlayerByID(accuserPlayerID)
	if accuser == nil {
		return ErrPlayerNotFound
	}
	target := g.findPlayerByID(targetPlayerID)
	if target == nil {
		return ErrPlayerNotFound
	}
	// grace period has not completed, reject the clainmmmmmm...
	if time.Now().UnixMilli() < g.unoGraceDeadline {
		return errors.New("domain: player is still within UNO grace period")
	}

	if g.vulnerablePlayerID != targetPlayerID || target.FinishedPlaying() || len(target.handCards) != 1 || target.HasSaidUno() {
		return errors.New("domain: player is not vulnerable to UNO penalty")
	}
	penaltyCards, err := g.DrawCardWithReshuffle(3)
	if err != nil {
		return err
	}
	target.AddToHand(penaltyCards)
	target.SetSaidUno(false)
	g.vulnerablePlayerID = -1
	g.unoGraceDeadline = 0
	g.lastAction = fmt.Sprintf("🚨 %s caught %s for not saying UNO! %s draws 3 cards!", accuser.name, target.name, target.name)
	return nil
}

func (g *GameState) findPlayerByID(playerID int) *Player {
	for _, p := range g.players {
		if p.id == playerID {
			return p
		}
	}
	return nil
}
