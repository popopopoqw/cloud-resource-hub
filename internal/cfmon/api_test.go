package cfmon

import "testing"

func TestNormalizeAPIToken(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "raw", input: "token-value", want: "token-value"},
		{name: "bearer", input: "Bearer token-value", want: "token-value"},
		{name: "case and spaces", input: "  bearer   token-value  ", want: "token-value"},
		{name: "full header", input: "Authorization: Bearer token-value", want: "token-value"},
		{name: "duplicated prefix", input: "Bearer Bearer token-value", want: "token-value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeAPIToken(tc.input); got != tc.want {
				t.Fatalf("NormalizeAPIToken() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewClientNormalizesAPIToken(t *testing.T) {
	client := NewClient("Authorization: Bearer token-value", "account")
	if client.apiToken != "token-value" {
		t.Fatalf("client token = %q, want token-value", client.apiToken)
	}
}
