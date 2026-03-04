package main

import (
	"encoding/hex"
	"testing"
)

func testKey() []byte {
	key, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey()
	original := "sk-ant-api03-secret-key-here"

	encrypted, err := encryptSecret(original, key)
	if err != nil {
		t.Fatalf("encryptSecret failed: %v", err)
	}

	if encrypted == original {
		t.Fatal("encrypted text should differ from original")
	}

	decrypted, err := decryptSecret(encrypted, key)
	if err != nil {
		t.Fatalf("decryptSecret failed: %v", err)
	}

	if decrypted != original {
		t.Fatalf("round-trip failed: got %q, want %q", decrypted, original)
	}
}

func TestEncryptDifferentNonces(t *testing.T) {
	key := testKey()
	plaintext := "same-secret"

	a, _ := encryptSecret(plaintext, key)
	b, _ := encryptSecret(plaintext, key)

	if a == b {
		t.Fatal("two encryptions of the same plaintext should produce different ciphertexts (random nonce)")
	}
}

func TestEncryptBadKeyLength(t *testing.T) {
	_, err := encryptSecret("secret", []byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestDecryptBadKeyLength(t *testing.T) {
	_, err := decryptSecret("aGVsbG8=", []byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestDecryptBadBase64(t *testing.T) {
	key := testKey()
	_, err := decryptSecret("not-valid-base64!!!", key)
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	key := testKey()
	encrypted, _ := encryptSecret("secret", key)

	// Tamper with the ciphertext
	tampered := encrypted[:len(encrypted)-2] + "XX"
	_, err := decryptSecret(tampered, key)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key := testKey()
	encrypted, _ := encryptSecret("secret", key)

	wrongKey, _ := hex.DecodeString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	_, err := decryptSecret(encrypted, wrongKey)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestParseEncryptionKeyValid(t *testing.T) {
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	key, err := parseEncryptionKey(hexKey)
	if err != nil {
		t.Fatalf("parseEncryptionKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(key))
	}
}

func TestParseEncryptionKeyEmpty(t *testing.T) {
	key, err := parseEncryptionKey("")
	if err != nil {
		t.Fatalf("expected nil error for empty key, got: %v", err)
	}
	if key != nil {
		t.Fatal("expected nil key for empty input")
	}
}

func TestParseEncryptionKeyBadHex(t *testing.T) {
	_, err := parseEncryptionKey("not-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestParseEncryptionKeyWrongLength(t *testing.T) {
	_, err := parseEncryptionKey("0123456789abcdef") // 8 bytes, not 32
	if err == nil {
		t.Fatal("expected error for wrong key length")
	}
}

func TestEncryptEmptyString(t *testing.T) {
	key := testKey()
	encrypted, err := encryptSecret("", key)
	if err != nil {
		t.Fatalf("encryptSecret empty string failed: %v", err)
	}

	decrypted, err := decryptSecret(encrypted, key)
	if err != nil {
		t.Fatalf("decryptSecret empty string failed: %v", err)
	}
	if decrypted != "" {
		t.Fatalf("expected empty string, got %q", decrypted)
	}
}

func TestEncryptLongString(t *testing.T) {
	key := testKey()
	long := string(make([]byte, 10000))
	encrypted, err := encryptSecret(long, key)
	if err != nil {
		t.Fatalf("encryptSecret long string failed: %v", err)
	}

	decrypted, err := decryptSecret(encrypted, key)
	if err != nil {
		t.Fatalf("decryptSecret long string failed: %v", err)
	}
	if decrypted != long {
		t.Fatal("round-trip failed for long string")
	}
}
