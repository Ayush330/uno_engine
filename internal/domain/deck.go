package domain

import (
	"crypto/rand"
	"math/big"
)

type Deck []Card

func NewDeck() Deck {
	deck := make(Deck, 0, 108)
	// create wild cards
	for range 4 {
		kindWild, err := NewWildCard(KindWild)
		if err != nil {
			panic(err)
		}
		deck = append(deck, kindWild)
		kindWildDrawFour, err := NewWildCard(KindWildDrawFour)
		if err != nil {
			panic(err)
		}
		deck = append(deck, kindWildDrawFour)
	}
	// create number cards
	for value := 0; value <= 9; value++ {
		var iteration int
		for color := Red; color <= Yellow; color++ {
			switch value {
			case 0:
				iteration = 1
			default:
				iteration = 2
			}

			for i := 0; i < iteration; i++ {
				card, err := NewNumberCard(color, Value(value))
				if err != nil {
					panic(err)
				}
				deck = append(deck, card)
			}
		}
	}
	// create action cards
	for color := Red; color <= Yellow; color++ {
		iteration := 2
		for i := 0; i < iteration; i++ {
			skipCard, err := NewActionCard(color, KindSkip)
			if err != nil {
				panic(err)
			}
			deck = append(deck, skipCard)
			reverseCard, err := NewActionCard(color, KindReverse)
			if err != nil {
				panic(err)
			}
			deck = append(deck, reverseCard)
			drawtwocard, err := NewActionCard(color, KindDrawTwo)
			if err != nil {
				panic(err)
			}
			deck = append(deck, drawtwocard)
		}
	}
	return deck
}

func (d *Deck) Shuffle() error {
	n := len(*d)
	for i := n - 1; i > 0; i-- {
		// Generate random integer j such that 0 <= j <= i
		// crypto/rand.Int returns [0, max), so we pass big.NewInt(int64(i + 1))
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		j := int(jBig.Int64())

		// Swap in-place
		(*d)[i], (*d)[j] = (*d)[j], (*d)[i]
	}
	return nil
}
