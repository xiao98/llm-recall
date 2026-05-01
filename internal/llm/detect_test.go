package llm

import (
	"errors"
	"testing"
)

func TestDetectKey(t *testing.T) {
	cases := []struct {
		name       string
		ant, oai   string
		wantVendor Vendor
		wantErr    bool
	}{
		{"only anthropic", "sk-ant-xxx", "", Anthropic, false},
		{"only openai", "", "sk-xxx", OpenAI, false},
		{"both → anthropic wins", "sk-ant-xxx", "sk-xxx", Anthropic, false},
		{"neither", "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ANTHROPIC_API_KEY", tc.ant)
			t.Setenv("OPENAI_API_KEY", tc.oai)
			v, _, err := DetectKey()
			if tc.wantErr {
				if !errors.Is(err, ErrNoKey) {
					t.Fatalf("want ErrNoKey, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v != tc.wantVendor {
				t.Errorf("vendor: got %s, want %s", v, tc.wantVendor)
			}
		})
	}
}

func TestKeyForVendor(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-foo")
	if _, err := KeyForVendor(Anthropic); !errors.Is(err, ErrNoKey) {
		t.Errorf("expected ErrNoKey for empty anthropic env, got %v", err)
	}
	k, err := KeyForVendor(OpenAI)
	if err != nil || k != "sk-foo" {
		t.Errorf("openai: got (%q,%v), want (sk-foo,nil)", k, err)
	}
}
