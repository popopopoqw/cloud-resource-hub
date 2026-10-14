package server

import (
	"io"
	"log"
	"time"

	"resource-center/internal/book"
	"resource-center/internal/cfmon"
	"resource-center/internal/config"
	"resource-center/internal/gmail"
	"resource-center/internal/mail"
	"resource-center/internal/model"
	"resource-center/internal/pan123web"
	"resource-center/internal/r2"
	"resource-center/internal/respool"
	"resource-center/internal/zlib"
)

var integrationSecretKeys = map[string]map[string]bool{
	"pan123":     {"token": true, "password": true},
	"forum":      {"cookie": true, "password": true},
	"gmail":      {"client_secret": true, "refresh_token": true},
	"cloudflare": {"api_token": true},
	"r2":         {"access_key_id": true, "secret_access_key": true},
	"library":    {"password": true},
}

func isIntegrationSecret(section, key string) bool {
	return integrationSecretKeys[section][key]
}

func setIntegrationConfigValue(cfg *config.Config, section, key, value string) {
	switch section + "/" + key {
	case "pan123/token":
		cfg.Pan123Token = value
	case "pan123/account":
		cfg.Pan123Account = value
	case "pan123/password":
		cfg.Pan123Password = value
	case "forum/cookie":
		cfg.ResPoolCookie = value
	case "forum/account":
		cfg.ResPoolAccount = value
	case "forum/password":
		cfg.ResPoolPassword = value
	case "gmail/client_id":
		cfg.GmailClientID = value
	case "gmail/client_secret":
		cfg.GmailClientSecret = value
	case "gmail/refresh_token":
		cfg.GmailRefreshToken = value
	case "gmail/email":
		cfg.GmailEmail = value
	case "cloudflare/api_token":
		cfg.CFAPIToken = value
	case "cloudflare/account_id":
		cfg.CFAccountID = value
	case "r2/account_id":
		cfg.R2AccountID = value
	case "r2/access_key_id":
		cfg.R2AccessKeyID = value
	case "r2/secret_access_key":
		cfg.R2SecretAccessKey = value
	case "r2/bucket":
		cfg.R2Bucket = value
	case "r2/public_base":
		cfg.R2PublicBase = value
	case "library/mode":
		cfg.ZLibMode = value
	case "library/base_url":
		cfg.ZLibBaseURL = value
	case "library/email":
		cfg.ZLibEmail = value
	case "library/password":
		cfg.ZLibPassword = value
	}
}

func (s *Server) configSnapshot() config.Config {
	s.servicesMu.RLock()
	defer s.servicesMu.RUnlock()
	return *s.cfg
}

func (s *Server) loadPersistedIntegrationSettings() {
	if s.settingsCipher == nil {
		return
	}
	items, err := s.store.ListIntegrationSettings()
	if err != nil {
		s.settingsErr = err
		log.Printf("读取 SQLite 服务配置失败: %v", err)
		return
	}
	for _, item := range items {
		value := item.Value
		if item.Secret {
			value, err = s.settingsCipher.Decrypt(item.Value)
			if err != nil {
				s.settingsErr = err
				log.Printf("解密 SQLite 配置 %s/%s 失败: %v", item.Section, item.Key, err)
				continue
			}
		}
		setIntegrationConfigValue(s.cfg, item.Section, item.Key, value)
	}
}

func (s *Server) persistIntegrationSettings(section string, values map[string]string) error {
	if s.settingsCipher == nil {
		return s.settingsErr
	}
	items := make([]model.IntegrationSetting, 0, len(values))
	for key, value := range values {
		secret := isIntegrationSecret(section, key)
		stored := value
		var err error
		if secret {
			stored, err = s.settingsCipher.Encrypt(value)
			if err != nil {
				return err
			}
		}
		items = append(items, model.IntegrationSetting{
			Section: section, Key: key, Value: stored, Secret: secret, UpdatedAt: nowUTC(),
		})
	}
	return s.store.SaveIntegrationSettings(section, items)
}

func nowUTC() (t time.Time) { return time.Now().UTC() }

func (s *Server) applyIntegrationValues(section string, values map[string]string) {
	s.servicesMu.Lock()
	for key, value := range values {
		setIntegrationConfigValue(s.cfg, section, key, value)
	}
	cfg := *s.cfg
	s.servicesMu.Unlock()
	s.applyIntegrationRuntime(section, cfg)
	if section == "cloudflare" {
		// Account ID 在 Cloudflare 页面修改后，R2 客户端也立即切到同一账户。
		s.applyIntegrationRuntime("r2", cfg)
	}
}

func (s *Server) applyAllIntegrationRuntimes() {
	cfg := s.configSnapshot()
	for _, section := range []string{"pan123", "forum", "gmail", "cloudflare", "r2", "library"} {
		s.applyIntegrationRuntime(section, cfg)
	}
}

func (s *Server) applyIntegrationRuntime(section string, cfg config.Config) {
	switch section {
	case "pan123":
		client := pan123web.New(cfg.Pan123Token, cfg.Pan123Account, cfg.Pan123Password)
		s.servicesMu.Lock()
		s.panWeb = client
		s.panFS = &panWebFS{c: client}
		s.servicesMu.Unlock()
	case "forum":
		pool := respool.New(cfg.ResPoolCookie, cfg.ResPoolAccount, cfg.ResPoolPassword, cfg.DataDir, cfg.ResPoolHeaded)
		s.servicesMu.Lock()
		s.pool = pool
		s.servicesMu.Unlock()
	case "gmail":
		registry := mail.NewRegistry()
		if cfg.GmailClientID != "" && cfg.GmailClientSecret != "" && cfg.GmailRefreshToken != "" {
			registry.Register(gmail.NewClient(cfg.GmailClientID, cfg.GmailClientSecret, cfg.GmailRefreshToken, cfg.GmailEmail))
		}
		s.servicesMu.Lock()
		s.mailReg = registry
		s.servicesMu.Unlock()
	case "cloudflare":
		accountID := cfg.CloudflareAccountID()
		var monitor cfmon.Monitor // 未配置保持 nil，接口层报「未配置」
		if cfg.CFAPIToken != "" && accountID != "" {
			monitor = cfmon.NewClient(cfg.CFAPIToken, accountID)
		}
		s.servicesMu.Lock()
		s.mon = monitor
		s.servicesMu.Unlock()
	case "r2":
		accountID := cfg.CloudflareAccountID()
		client := r2.New(accountID, cfg.R2AccessKeyID, cfg.R2SecretAccessKey, cfg.R2Bucket)
		var objectStore ObjectStore = NewLocalStore(cfg.DataDir)
		if client.Configured() {
			objectStore = r2ObjectStore{client: client}
		}
		s.servicesMu.Lock()
		s.obj = objectStore
		s.servicesMu.Unlock()
	case "library":
		var provider book.Provider
		switch cfg.ZLibMode {
		case "direct":
			provider = zlib.New(zlib.Options{BaseURL: cfg.ZLibBaseURL, Email: cfg.ZLibEmail, Password: cfg.ZLibPassword})
		default: // jump（默认）：真实入口，无演示降级
			provider = zlib.NewJump(cfg.ZLibBaseURL)
		}
		s.servicesMu.Lock()
		s.books = provider
		s.servicesMu.Unlock()
	}
}

type r2ObjectStore struct{ client *r2.Client }

func (s r2ObjectStore) Kind() string { return "r2" }
func (s r2ObjectStore) PutObject(key string, reader io.Reader, contentType string, size int64) (int64, error) {
	return s.client.PutObject(key, reader, contentType, size)
}
func (s r2ObjectStore) GetObject(key string) (io.ReadCloser, int64, string, error) {
	return s.client.GetObject(key)
}

func (s *Server) bookProvider() book.Provider {
	s.servicesMu.RLock()
	defer s.servicesMu.RUnlock()
	return s.books
}
func (s *Server) resourcePool() *respool.Fetcher {
	s.servicesMu.RLock()
	defer s.servicesMu.RUnlock()
	return s.pool
}
func (s *Server) panClient() *pan123web.Client {
	s.servicesMu.RLock()
	defer s.servicesMu.RUnlock()
	return s.panWeb
}
func (s *Server) driveFileSystem() *panWebFS {
	s.servicesMu.RLock()
	defer s.servicesMu.RUnlock()
	return s.panFS
}
func (s *Server) mailRegistry() *mail.Registry {
	s.servicesMu.RLock()
	defer s.servicesMu.RUnlock()
	return s.mailReg
}
func (s *Server) monitor() cfmon.Monitor {
	s.servicesMu.RLock()
	defer s.servicesMu.RUnlock()
	return s.mon
}
func (s *Server) objectStore() ObjectStore {
	s.servicesMu.RLock()
	defer s.servicesMu.RUnlock()
	return s.obj
}
