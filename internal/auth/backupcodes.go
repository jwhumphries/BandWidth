package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const backupCodeCount = 10

// NewBackupCodes returns one-time recovery codes like "4F7K-Q2ML".
// The alphabet omits 0/O and 1/I to avoid transcription mistakes.
func NewBackupCodes() []string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	codes := make([]string, backupCodeCount)
	for i := range codes {
		buf := make([]byte, 8)
		for j := range buf {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			if err != nil {
				panic(err) // crypto/rand failure is unrecoverable
			}
			buf[j] = alphabet[n.Int64()]
		}
		codes[i] = fmt.Sprintf("%s-%s", buf[:4], buf[4:])
	}
	return codes
}
