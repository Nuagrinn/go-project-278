package main

import (
	"crypto/rand"
	"math/big"
	"regexp"
)

const (
	defaultShortNameLength = 7
	maxGenerateAttempts    = 5
)

var (
	shortNameAlphabet = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	shortNamePattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{3,64}$`)
)

func generateShortName(length int) (string, error) {
	result := make([]rune, length)
	alphabetSize := big.NewInt(int64(len(shortNameAlphabet)))

	for i := range result {
		index, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", err
		}

		result[i] = shortNameAlphabet[index.Int64()]
	}

	return string(result), nil
}

func isValidShortName(value string) bool {
	return shortNamePattern.MatchString(value)
}
