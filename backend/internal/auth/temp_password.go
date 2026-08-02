package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// GenerateTempPassword returns a 6-digit numeric password, easy to type
// after receiving it by SMS/WhatsApp.
func GenerateTempPassword() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
