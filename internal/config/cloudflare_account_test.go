package config

import "testing"

func TestCloudflareAccountIDPrefersSharedID(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "shared", cfg: Config{CFAccountID: "shared", R2AccountID: "legacy"}, want: "shared"},
		{name: "legacy fallback", cfg: Config{R2AccountID: "legacy"}, want: "legacy"},
		{name: "unconfigured", cfg: Config{}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.CloudflareAccountID(); got != tc.want {
				t.Fatalf("CloudflareAccountID() = %q, want %q", got, tc.want)
			}
		})
	}
}
