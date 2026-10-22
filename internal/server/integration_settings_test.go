package server

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"resource-center/internal/config"
	"resource-center/internal/store"
)

func TestPersistIntegrationSettingsEncryptsOnlySecrets(t *testing.T) {
	cipher, err := newCredentialCipher(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mem := store.NewMemStore()
	s := &Server{store: mem, settingsCipher: cipher}
	if err := s.persistIntegrationSettings("gmail", map[string]string{
		"email": "person@example.com", "client_secret": "top-secret", "refresh_token": "refresh-secret",
	}); err != nil {
		t.Fatal(err)
	}
	items, err := mem.ListIntegrationSettings()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d", len(items))
	}
	for _, item := range items {
		switch item.Key {
		case "email":
			if item.Secret || item.Value != "person@example.com" {
				t.Fatalf("email should remain plaintext: %#v", item)
			}
		case "client_secret", "refresh_token":
			if !item.Secret || !strings.HasPrefix(item.Value, encryptedValuePrefix) || strings.Contains(item.Value, "secret") {
				t.Fatalf("secret should be encrypted: %#v", item)
			}
			if _, err := cipher.Decrypt(item.Value); err != nil {
				t.Fatalf("decrypt %s: %v", item.Key, err)
			}
		}
	}
}

func TestSavedPhoneAndEmailReappearInSettings(t *testing.T) {
	dataDir := t.TempDir()
	cipher, err := newCredentialCipher(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.NewSQLiteStore(filepath.Join(dataDir, "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Server{cfg: &config.Config{}, store: db, settingsCipher: cipher}
	for _, tc := range []struct {
		section string
		key     string
		value   string
	}{
		{"pan123", "account", "13800138000"},
		{"forum", "account", "forum@example.com"},
		{"gmail", "email", "gmail@example.com"},
		{"library", "email", "library@example.com"},
	} {
		if err := s.persistIntegrationSettings(tc.section, map[string]string{tc.key: tc.value}); err != nil {
			t.Fatal(err)
		}
	}
	// 模拟服务重启后从 SQLite 读取已保存的配置。
	restarted := &Server{cfg: &config.Config{}, store: db, settingsCipher: cipher}
	restarted.loadPersistedIntegrationSettings()
	w := httptest.NewRecorder()
	restarted.handleIntegrationSettings(w, httptest.NewRequest("GET", "/api/v1/settings/integrations", nil))
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	var response struct {
		Data map[string]map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		section string
		key     string
		want    string
	}{
		{"pan123", "account", "13800138000"},
		{"forum", "account", "forum@example.com"},
		{"gmail", "email", "gmail@example.com"},
		{"library", "email", "library@example.com"},
	} {
		if got := response.Data[tc.section][tc.key]; got != tc.want {
			t.Errorf("%s/%s = %v, want %q", tc.section, tc.key, got, tc.want)
		}
	}
}

func TestRequestOriginPrefersConfiguredBaseURL(t *testing.T) {
	req := httptest.NewRequest("GET", "http://internal:8080/path", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := requestOrigin("https://public.example.com/", req); got != "https://public.example.com" {
		t.Fatalf("configured origin = %q", got)
	}
	if got := requestOrigin("", req); got != "https://internal:8080" {
		t.Fatalf("request origin = %q", got)
	}
}

func TestIntegrationSecretClassification(t *testing.T) {
	for _, tc := range []struct {
		section string
		key     string
		want    bool
	}{
		{"pan123", "password", true}, {"pan123", "token", true},
		{"forum", "cookie", true}, {"gmail", "refresh_token", true},
		{"gmail", "client_secret", true}, {"cloudflare", "api_token", true},
		{"r2", "access_key_id", true}, {"r2", "secret_access_key", true},
		{"pan123", "account", false}, {"gmail", "email", false},
		{"library", "email", false}, {"cloudflare", "account_id", false},
	} {
		if got := isIntegrationSecret(tc.section, tc.key); got != tc.want {
			t.Errorf("isIntegrationSecret(%q,%q)=%v want %v", tc.section, tc.key, got, tc.want)
		}
	}
}

func TestR2ReusesCloudflareAccountID(t *testing.T) {
	if _, ok := integrationEnvKeys["r2"]["account_id"]; ok {
		t.Fatal("R2 must not expose a separate Account ID setting")
	}
	cfg := &config.Config{
		R2AccountID:       "legacy-account",
		R2AccessKeyID:     "access-key",
		R2SecretAccessKey: "secret-key",
		R2Bucket:          "books",
		DataDir:           t.TempDir(),
	}
	s := &Server{cfg: cfg, obj: NewLocalStore(cfg.DataDir)}
	if got := s.objectStore().Kind(); got != "local" {
		t.Fatalf("initial storage kind = %q", got)
	}
	s.applyIntegrationValues("cloudflare", map[string]string{"account_id": "shared-account"})
	if got := s.configSnapshot().CloudflareAccountID(); got != "shared-account" {
		t.Fatalf("shared account ID = %q", got)
	}
	if got := s.objectStore().Kind(); got != "r2" {
		t.Fatalf("R2 runtime was not refreshed after Cloudflare Account ID change: %q", got)
	}
}
