package utils

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor for bcrypt hashing
	// 12 is a good balance between security and performance
	BcryptCost = 12
)

// HashPassword generates a bcrypt hash from a plaintext password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword compares a plaintext password with a bcrypt hash
// Returns true if they match
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidatePasswordStrength checks if password meets minimum requirements
func ValidatePasswordStrength(password string) bool {
	// Minimum 6 characters (same as Payments)
	return len(password) >= 6
}
