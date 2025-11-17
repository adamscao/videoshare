package utils

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
)

// GenerateSlug generates a random URL-safe slug
func GenerateSlug(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	// Base64 URL encoding and remove padding
	slug := base64.URLEncoding.EncodeToString(b)
	slug = strings.ReplaceAll(slug, "=", "")
	slug = strings.ReplaceAll(slug, "+", "")
	slug = strings.ReplaceAll(slug, "/", "")

	// Trim to desired length
	if len(slug) > length {
		slug = slug[:length]
	}

	return slug, nil
}
