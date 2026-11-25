package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory  = 19456 // KiB (OWASP minimum for argon2id)
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

var (
	argonActive, argonPeak atomic.Int64
	// argonSlots caps concurrent hashes: each one holds ~19 MiB, so a burst of
	// login requests must queue instead of exhausting memory.
	argonSlots = make(chan struct{}, 4)
)

// argonKey runs argon2id in one of the limited slots, tracking how many run at once.
func argonKey(pw, salt []byte, t, m uint32, p uint8, n uint32) []byte {
	argonSlots <- struct{}{}
	defer func() { <-argonSlots }()
	cur := argonActive.Add(1)
	defer argonActive.Add(-1)
	for {
		peak := argonPeak.Load()
		if cur <= peak || argonPeak.CompareAndSwap(peak, cur) {
			break
		}
	}
	return argon2.IDKey(pw, salt, t, m, p, n)
}

// HashPassword returns a PHC-formatted argon2id hash.
func HashPassword(pw string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argonKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// VerifyPassword checks pw against a hash produced by HashPassword.
func VerifyPassword(encoded, pw string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("unsupported password hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("unsupported argon2 version")
	}
	var mem, iters uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iters, &threads); err != nil {
		return false, err
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := enc.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got := argonKey([]byte(pw), salt, iters, mem, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummyHash keeps login timing similar for unknown usernames.
var dummyHash, _ = HashPassword("patchboard-dummy-password")
