package mail

import (
	"encoding/base64"
	"strings"
	"testing"
)

const (
	testKey  = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	otherKey = "1f1e1d1c1b1a191817161514131211100f0e0d0c0b0a09080706050403020100"
)

func TestCryptoRoundTrip(t *testing.T) {
	stored, err := Encrypt(testKey, "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if stored == "hunter2" || strings.Contains(stored, "hunter2") {
		t.Fatal("must not be stored in the clear")
	}
	got, err := Decrypt(testKey, stored)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hunter2" {
		t.Fatalf("decrypt = %q", got)
	}
}

func TestCryptoFreshNonce(t *testing.T) {
	a, err := Encrypt(testKey, "same")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Encrypt(testKey, "same")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("identical ciphertext would leak equality")
	}
}

func TestCryptoWrongKeyAndTampering(t *testing.T) {
	stored, err := Encrypt(testKey, "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(otherKey, stored); err == nil {
		t.Fatal("wrong key must be rejected")
	}
	blob, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		t.Fatal(err)
	}
	blob[len(blob)-1] ^= 0x01
	if _, err := Decrypt(testKey, base64.StdEncoding.EncodeToString(blob)); err == nil {
		t.Fatal("tampering must be detected")
	}
}

func TestCryptoRejectsBadKeys(t *testing.T) {
	if _, err := Encrypt("abcd", "x"); err == nil {
		t.Fatal("short key must be rejected")
	}
	if _, err := Encrypt(strings.Repeat("z", 64), "x"); err == nil {
		t.Fatal("non-hex key must be rejected")
	}
	short := base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4})
	if _, err := Decrypt(testKey, short); err == nil {
		t.Fatal("truncated blob must be rejected")
	}
}
