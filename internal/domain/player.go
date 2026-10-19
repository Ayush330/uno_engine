package domain

import "errors"

var (
	ErrCardNotFound = errors.New("domain: card not found in player's hand")
)

type Player struct {
	name      string // Name of the player
	handCards []Card // hand cards for the player
	id        int    // seat index for player
}

func CreateNewPlayer(name string, id int) *Player {
	return &Player{
		name:      name,
		handCards: make([]Card, 0),
		id:        id,
	}
}

func (p *Player) AddCardToHand(card Card) {
	p.handCards = append(p.handCards, card)
}

func (p *Player) RemoveCardFromHand(card Card) error {
	for i, c := range p.handCards {
		if c == card {
			// found the card
			p.handCards = append(p.handCards[:i], p.handCards[i+1:]...)
			return nil
		}
	}
	return ErrCardNotFound
}

func (p *Player) AddToHand(cards []Card) {
	p.handCards = append(p.handCards, cards...)
}
