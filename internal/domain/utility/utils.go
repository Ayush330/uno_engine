package utility

import (
	"math/rand"
)

const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const length = 4

func GenerateRoomId() string {
	code := make([]byte, length)
	for i := 0; i < length; i++ {
		randIndex := rand.Intn(len(charset))
		code[i] = charset[randIndex]
	}
	return string(code)
}

func GenerateIntInBetweenLimits(min, max int) int {
	return rand.Intn(max-min+1) + min
}
