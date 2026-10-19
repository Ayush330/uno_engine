package domain

import (
	"errors"
)

var (
	ErrDiscardPileEmpty          = errors.New("error: discard pile is empty")
	ErrIllegalMove               = errors.New("domain: card cannot be played on discard pile")
	ErrInitializationDiscardPile = errors.New("error: discard pile cannot be initialized with action card")
)

type DiscardPile struct {
	cards       []Card
	activeColor Color
}

// Currently I am taking only a NumbererdCard as a discard pile
// As after shuffling and distributing the deck we pick a random card to be the top card

func NewDiscardPile(card Card) (*DiscardPile, error) {
	if card.Kind() == KindNumber && card.IsValid() {
		return &DiscardPile{
			cards:       []Card{card},
			activeColor: card.Color(),
		}, nil
	}
	return nil, ErrInitializationDiscardPile
}

func (d *DiscardPile) LastCard() (Card, error) {
	if len(d.cards) == 0 {
		return Card{}, ErrDiscardPileEmpty
	}
	return d.cards[len(d.cards)-1], nil
}

func (d *DiscardPile) ActiveColor() Color {
	return d.activeColor
}

func (d *DiscardPile) UpdateState(card Card, color Color) error {
	IsStateConsistent, err := d.IsStateConsistent(card, color)
	if err != nil {
		return err
	}
	if !IsStateConsistent {
		return ErrIllegalMove
	}
	d.cards = append(d.cards, card)
	switch card.Kind() {
	case KindWild, KindWildDrawFour:
		d.activeColor = color
	default:
		d.activeColor = card.Color()
	}
	return nil
}

func (d *DiscardPile) IsStateConsistent(card Card, color Color) (bool, error) {
	//Validate card
	if !card.IsValid() {
		return false, ErrInvalidCard
	}

	if !color.IsValid() {
		return false, ErrInvalidColor
	}
	// Logic is that
	// if it is number Cards then either last card should have the same color or number,
	// if it is KindSkip, KindReverse, KindDrawTwo then the color shuould match or the action should match
	// for the remaining cards no match is needed

	if len(d.cards) == 0 {
		return false, errors.New("error: discard pile is empty")
	}

	TopCard := d.cards[len(d.cards)-1]

	switch card.Kind() {
	case KindNumber:
		return card.Color() == d.activeColor || (card.Value() == TopCard.Value() && TopCard.Kind() == KindNumber), nil
	case KindDrawTwo, KindReverse, KindSkip:
		return card.Color() == d.activeColor || card.Kind() == TopCard.Kind(), nil
	case KindWild, KindWildDrawFour:
		if !color.IsValidWildCardColor() {
			return false, ErrInvalidColor
		}
		return true, nil
	default:
		return false, ErrInvalidCard
	}

}
