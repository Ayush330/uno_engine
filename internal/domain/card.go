package domain

import (
	"errors"
	"fmt"
)

type Color uint8
type Value uint8

const (
	NoColor Color = iota
	Red
	Blue
	Green
	Yellow
)

type Kind uint8

const (
	KindNumber Kind = iota + 1
	KindSkip
	KindReverse
	KindDrawTwo
	KindWild
	KindWildDrawFour
)

//fmt.Stringer implement it

func (c Color) String() string {
	switch c {
	case NoColor:
		return "NoColor"
	case Red:
		return "Red"
	case Blue:
		return "Blue"
	case Green:
		return "Green"
	case Yellow:
		return "Yellow"
	default:
		return "Unknown Color"
	}
}

func (k Kind) String() string {
	switch k {
	case KindNumber:
		return "Number"
	case KindSkip:
		return "Skip"
	case KindReverse:
		return "Reverse"
	case KindDrawTwo:
		return "DrawTwo"
	case KindWild:
		return "Wild"
	case KindWildDrawFour:
		return "WildDrawFour"
	default:
		return "Unknown Kind"
	}
}

type Card struct {
	CardColor Color `json:"color"`
	CardKind  Kind  `json:"kind"`
	CardValue Value `json:"value"`
}

type ValidCard interface {
	IsValid() bool
}

func (c Card) String() string {
	switch c.CardKind {
	case KindNumber:
		return fmt.Sprintf("%s %d", c.CardColor, c.CardValue)
	case KindSkip, KindReverse, KindDrawTwo:
		return fmt.Sprintf("%s %s", c.CardColor, c.CardKind)
	case KindWild, KindWildDrawFour:
		return c.CardKind.String()
	default:
		return "Invalid Card"
	}
}

var (
	ErrInvalidCard  = errors.New("domain: invalid card - check for color, kind or value")
	ErrInvalidColor = errors.New("domain: invalid color")
	ErrInvalidKind  = errors.New("domain: invalid card kind")
	ErrInvalidValue = errors.New("domain: number must be between 0 and 9")
)

func (c Card) Color() Color {
	return c.CardColor
}

func (c Card) Value() Value {
	return c.CardValue
}

func (c Card) Kind() Kind {
	return c.CardKind
}

func (c Card) IsValid() bool {
	// Color Check, Kind Check and Value Check
	return c.Color().IsValid() && c.Kind().IsValid() && c.Value().IsValid()
}

func (c Color) IsValid() bool {
	return c >= NoColor && c <= Yellow
}

func (c Color) IsValidWildCardColor() bool {
	return c >= Red && c <= Yellow
}

func (k Kind) IsValid() bool {
	switch k {
	case KindNumber, KindSkip, KindReverse, KindDrawTwo, KindWild, KindWildDrawFour:
		return true
	default:
		return false
	}
}

func (v Value) IsValid() bool {
	return !(v > 9)
}

func NewNumberCard(color Color, value Value) (Card, error) {
	if color < Red || color > Yellow {
		return Card{}, ErrInvalidColor
	}

	if value > 9 {
		return Card{}, ErrInvalidValue
	}

	return Card{CardColor: color, CardKind: KindNumber, CardValue: value}, nil
}

func NewActionCard(color Color, kind Kind) (Card, error) {
	if color < Red || color > Yellow {
		return Card{}, ErrInvalidColor
	}

	switch kind {
	case KindSkip, KindReverse, KindDrawTwo:
		return Card{CardColor: color, CardKind: kind, CardValue: 0}, nil
	default:
		return Card{}, ErrInvalidKind
	}
}

func NewWildCard(kind Kind) (Card, error) {
	switch kind {
	case KindWild, KindWildDrawFour:
		return Card{CardColor: NoColor, CardKind: kind, CardValue: 0}, nil
	default:
		return Card{}, ErrInvalidKind
	}
}

// We are now implementing CanPlayOn function.
// This means that can this card be played upon the current topCard

func (c Card) CanPlayOn(topCard Card) bool {
	switch c.CardKind {
	case KindNumber:
		if c.Color() == topCard.Color() {
			return true
		}
		return topCard.Kind() == KindNumber && c.Value() == topCard.Value()

	case KindSkip, KindDrawTwo, KindReverse:
		if c.Color() == topCard.Color() || c.Kind() == topCard.Kind() {
			return true
		} else {
			return false
		}
	case KindWild, KindWildDrawFour:
		return true
	default:
		return false
	}
}
