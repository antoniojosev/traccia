// Package password is Traccia's default PasswordHasher: bcrypt, the right
// tool for low-entropy human passwords (unlike internal/adapters/apikey,
// which hashes high-entropy random tokens with fast SHA-256 on purpose).
package password

import "golang.org/x/crypto/bcrypt"

type BcryptHasher struct{}

func NewBcryptHasher() *BcryptHasher {
	return &BcryptHasher{}
}

func (BcryptHasher) Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(hash), err
}

func (BcryptHasher) Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
