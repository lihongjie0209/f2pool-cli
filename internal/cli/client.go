package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func send(cmd *cobra.Command, path string, body map[string]any, opt *options) error {
	if !strings.HasPrefix(path, "/v2/") || strings.ContainsAny(path, "?#") || strings.Contains(path, "..") || strings.Contains(path, "//") {
		return fmt.Errorf("path must be a /v2/ API route")
	}
	for _, r := range path {
		if !(r == '/' || r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return fmt.Errorf("invalid character in API path")
		}
	}
	base, err := url.Parse(opt.baseURL)
	if err != nil || base == nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" || base.Path != "" {
		return fmt.Errorf("--base-url must be an HTTP(S) origin without path or query")
	}
	if base.Scheme == "http" && base.Hostname() != "localhost" && base.Hostname() != "127.0.0.1" && base.Hostname() != "::1" {
		return fmt.Errorf("HTTP base URL is allowed only for localhost")
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	if opt.dryRun {
		var pretty bytes.Buffer
		_ = json.Indent(&pretty, encoded, "", "  ")
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "POST %s%s\n%s\n", strings.TrimRight(opt.baseURL, "/"), path, pretty.Bytes())
		return err
	}
	token := os.Getenv("F2POOL_API_TOKEN")
	if token == "" {
		return fmt.Errorf("F2POOL_API_TOKEN is required")
	}
	request, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, strings.TrimRight(opt.baseURL, "/")+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("F2P-API-SECRET", token)
	client := &http.Client{
		Timeout: opt.timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(raw)))
	}
	var apiError struct {
		Code    json.Number `json:"code"`
		Message string      `json:"msg"`
	}
	if json.Unmarshal(raw, &apiError) == nil && apiError.Code != "" && apiError.Code != "0" {
		return fmt.Errorf("API error %s: %s", apiError.Code, apiError.Message)
	}
	if opt.output == "raw" {
		_, err = cmd.OutOrStdout().Write(raw)
		return err
	}
	if !json.Valid(raw) {
		return fmt.Errorf("API returned invalid JSON: %s", strings.TrimSpace(string(raw)))
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return err
	}
	pretty.WriteByte('\n')
	_, err = io.Copy(cmd.OutOrStdout(), &pretty)
	return err
}
