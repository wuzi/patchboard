package auth_test

import (
	"strings"
	"sync"
	"testing"

	"patchboard/internal/auth"
)

func TestPassword_RoundTrip(t *testing.T) {
	h, err := auth.HashPassword("correct-horse-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash = %s", h)
	}
	if ok, err := auth.VerifyPassword(h, "correct-horse-1"); err != nil || !ok {
		t.Fatalf("verify correct = %v %v", ok, err)
	}
	if ok, _ := auth.VerifyPassword(h, "wrong"); ok {
		t.Fatal("wrong password verified")
	}
	h2, _ := auth.HashPassword("correct-horse-1")
	if h2 == h {
		t.Fatal("hashes must be salted")
	}
}

func TestPassword_ArgonConcurrencyIsCapped(t *testing.T) {
	h, _ := auth.HashPassword("correct-horse-1")
	auth.ResetArgonPeak()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); auth.VerifyPassword(h, "wrong") }()
	}
	wg.Wait()
	if p := auth.ArgonPeak(); p > 4 {
		t.Fatalf("%d argon2 computations ran at once; cap is 4", p)
	}
}
