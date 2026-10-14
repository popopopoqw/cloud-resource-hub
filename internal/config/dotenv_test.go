package config

import (
	"os"
	"path/filepath"
	"testing"
)

// 环境变量优先级：环境变量存在（含空串）则 .env 不覆盖；不存在才采用 .env 值。
func TestLoadDotEnvPriority(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "A_RC_TEST_FROM_ENV_FILE=yes\n" +
		"A_RC_TEST_EMPTY_IN_ENV=\n" +
		"A_RC_TEST_ONLY_FILE=file-value\n" +
		"# A_RC_TEST_COMMENT=ignored\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("A_RC_TEST_FROM_ENV_FILE", "env-wins") // 环境变量已存在 → 不覆盖
	t.Setenv("A_RC_TEST_EMPTY_IN_ENV", "")          // 显式空串 → 同样视为存在，不覆盖

	LoadDotEnv(path)

	if got := os.Getenv("A_RC_TEST_FROM_ENV_FILE"); got != "env-wins" {
		t.Errorf("环境变量应优先: got %q, want %q", got, "env-wins")
	}
	if got := os.Getenv("A_RC_TEST_EMPTY_IN_ENV"); got != "" {
		t.Errorf("环境变量显式空串不应被 .env 覆盖: got %q", got)
	}
	if got := os.Getenv("A_RC_TEST_ONLY_FILE"); got != "file-value" {
		t.Errorf("环境变量不存在时应采用 .env 值: got %q, want %q", got, "file-value")
	}
	if _, exists := os.LookupEnv("A_RC_TEST_COMMENT"); exists {
		t.Errorf("注释行不应被加载")
	}
}
