package auth

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor for bcrypt hashing
	// Higher values = more secure but slower
	// 12 is a good balance for 2024 (takes ~250ms on modern hardware)
	BcryptCost = 12
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	if len(password) == 0 {
		return "", fmt.Errorf("password cannot be empty")
	}

	if len(password) > 72 {
		return "", fmt.Errorf("password too long (max 72 characters)")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hash), nil
}

// VerifyPassword compares a password with its hash
func VerifyPassword(password, hash string) error {
	if len(password) == 0 {
		return fmt.Errorf("password cannot be empty")
	}

	if len(hash) == 0 {
		return fmt.Errorf("hash cannot be empty")
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return fmt.Errorf("invalid password")
		}
		return fmt.Errorf("failed to verify password: %w", err)
	}

	return nil
}

// ValidatePasswordStrength validates password strength
func ValidatePasswordStrength(password string) error {
	const (
		minLength = 12
		maxLength = 72
	)

	if len(password) < minLength {
		return fmt.Errorf("password must be at least %d characters", minLength)
	}

	if len(password) > maxLength {
		return fmt.Errorf("password must be at most %d characters", maxLength)
	}

	// Check for complexity requirements
	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	specialChars := "!@#$%^&*()-_=+[]{}|;:,.<>?/~`"

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasNumber = true
		default:
			// Check if it's a special character
			for _, special := range specialChars {
				if char == special {
					hasSpecial = true
					break
				}
			}
		}
	}

	// Build detailed error message
	var missing []string
	if !hasUpper {
		missing = append(missing, "uppercase letter")
	}
	if !hasLower {
		missing = append(missing, "lowercase letter")
	}
	if !hasNumber {
		missing = append(missing, "number")
	}
	if !hasSpecial {
		missing = append(missing, "special character")
	}

	if len(missing) > 0 {
		return fmt.Errorf("password must contain at least one: %s", strings.Join(missing, ", "))
	}

	return nil
}

// CalculatePasswordStrength returns a password strength score (0-100)
func CalculatePasswordStrength(password string) int {
	score := 0

	// Length score (max 40 points)
	length := len(password)
	score += min(length*2, 40)

	// Variety score (max 60 points - 15 each for upper/lower/number/special)
	var hasUpper, hasLower, hasNumber, hasSpecial bool
	for _, char := range password {
		if char >= 'A' && char <= 'Z' {
			hasUpper = true
		} else if char >= 'a' && char <= 'z' {
			hasLower = true
		} else if char >= '0' && char <= '9' {
			hasNumber = true
		} else {
			hasSpecial = true
		}
	}

	if hasUpper {
		score += 15
	}
	if hasLower {
		score += 15
	}
	if hasNumber {
		score += 15
	}
	if hasSpecial {
		score += 15
	}

	return min(score, 100)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
