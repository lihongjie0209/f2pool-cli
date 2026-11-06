package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEndpointRequest(t *testing.T) {
	t.Setenv("F2POOL_API_TOKEN", "test-secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/hash_rate/info_list" {
			t.Errorf("request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("F2P-API-SECRET") != "test-secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("headers: %v", r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		reqs, ok := body["reqs"].([]any)
		if !ok || len(reqs) != 2 {
			t.Errorf("reqs: %#v", body["reqs"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"info":[]}`))
	}))
	defer server.Close()
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--base-url", server.URL, "hash-rate", "info-list", "--reqs", `{"currency":"bitcoin","mining_user_name":"alice"}`, "--reqs", `{"currency":"litecoin","mining_user_name":"bob"}`})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"info": []`) {
		t.Fatal(out.String())
	}
}

func TestAPIError(t *testing.T) {
	t.Setenv("F2POOL_API_TOKEN", "test-secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":401,"msg":"unauthorized"}`))
	}))
	defer server.Close()
	cmd := New()
	cmd.SetArgs([]string{"--base-url", server.URL, "mining-user", "list"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "API error 401") {
		t.Fatalf("error = %v", err)
	}
}

func TestDryRunWithoutToken(t *testing.T) {
	t.Setenv("F2POOL_API_TOKEN", "")
	cmd := New()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--dry-run", "assets", "balance", "--currency", "bitcoin", "--calculate-estimated-income"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"calculate_estimated_income": true`) {
		t.Fatal(out.String())
	}
}

func TestV1MultiAccount(t *testing.T) {
	t.Setenv("F2POOL_API_TOKEN", "legacy-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/bitcoin/alice" {
			t.Errorf("request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("F2P-API-SECRET") != "legacy-token" {
			t.Errorf("missing token header")
		}
		accounts := r.URL.Query()["multi_account"]
		if len(accounts) != 2 || accounts[0] != "alice" || accounts[1] != "bob" {
			t.Errorf("accounts: %v", accounts)
		}
		_, _ = w.Write([]byte(`{"accounts":[]}`))
	}))
	defer server.Close()
	cmd := New()
	cmd.SetArgs([]string{"--base-url", server.URL, "v1", "multi-account", "--currency", "bitcoin", "--user", "alice", "--user", "bob"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
