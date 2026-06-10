// Package auth holds authentication primitives: password hashing, random
// tokens, TOTP, and backup codes.
package auth

import "github.com/alexedwards/argon2id"

// HashPassword hashes a password with argon2id default parameters.
func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

// VerifyPassword reports whether password matches the stored hash.
func VerifyPassword(password, hash string) bool {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	return err == nil && match
}
