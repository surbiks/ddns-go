package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/rand"
	"time"
)

// GenerateToken Token
func GenerateToken(username string) string {
	key := []byte(generateRandomKey())
	h := hmac.New(sha256.New, key)
	msg := fmt.Sprintf("%s%d", username, time.Now().Unix())
	h.Write([]byte(msg))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// generateRandomKey generate random key
func generateRandomKey() string {
	// set random seed
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)

	// generate random 64-bit integer
	randomNumber := random.Uint64()

	return fmt.Sprint(randomNumber)
}
