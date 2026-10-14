package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentialCipherRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cipher, err := newCredentialCipher(dir)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cipher.Encrypt("secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encrypted, encryptedValuePrefix) || strings.Contains(encrypted, "secret-token") {
		t.Fatalf("unexpected encrypted value: %q", encrypted)
	}
	plain, err := cipher.Decrypt(encrypted)
	if err != nil || plain != "secret-token" {
		t.Fatalf("round trip failed: plain=%q err=%v", plain, err)
	}
	info, err := os.Stat(filepath.Join(dir, "integration-settings.key"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key permissions = %o", info.Mode().Perm())
	}
}
