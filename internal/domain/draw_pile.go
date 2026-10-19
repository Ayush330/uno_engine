package domain

import (
	"errors"
)

type DrawPile struct {
	cards []Card
}

var (
	ErrInsufficientCards               = errors.New("error: not enough cards in draw pile")
	ErrInitializationFailedForDrawPile = errors.New("error: draw pile cannot be initialized with invalid cards")
)

func NewDrawPile(cards []Card) (*DrawPile, error) {
	for _, card := range cards {
		if !card.IsValid() {
			return nil, ErrInitializationFailedForDrawPile
		}
	}
	copied := make([]Card, len(cards))
	copy(copied, cards)
	return &DrawPile{
		cards: copied,
	}, nil
}

// pick cards from draw pile
func (d *DrawPile) DrawCards(count uint8) ([]Card, error) {
	if len(d.cards) < int(count) {
		return nil, ErrInsufficientCards
	}
	drawn := d.cards[:count]
	d.cards = d.cards[count:]
	return drawn, nil
}
