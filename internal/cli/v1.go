package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func v1Command(opt *options) *cobra.Command {
	group := &cobra.Command{Use: "v1", Short: "Legacy read-only GET API (requires legacy/api_v1 token permission)"}
	var currency, user, worker string
	var history int
	account := &cobra.Command{Use: "account", Short: "Account balance, revenue, payouts, workers and hashrate", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if currency == "" || user == "" {
				return fmt.Errorf("--currency and --user are required")
			}
			if cmd.Flags().Changed("history") && (history < 1 || history > 60) {
				return fmt.Errorf("--history must be 1 through 60 days")
			}
			query := url.Values{}
			if cmd.Flags().Changed("history") {
				query.Set("history", strconv.Itoa(history))
			}
			return sendV1(cmd, opt, []string{currency, user}, query)
		},
	}
	account.Flags().StringVar(&currency, "currency", "", "Mining currency")
	account.Flags().StringVar(&user, "user", "", "Mining account")
	account.Flags().IntVar(&history, "history", 0, "Account hashrate history in days (1-60)")
	group.AddCommand(account)

	workerCmd := &cobra.Command{Use: "worker", Short: "Worker hashrate history for the past 24 hours", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if currency == "" || user == "" || worker == "" {
				return fmt.Errorf("--currency, --user and --worker-name are required")
			}
			return sendV1(cmd, opt, []string{currency, user, worker}, nil)
		},
	}
	workerCmd.Flags().StringVar(&currency, "currency", "", "Mining currency")
	workerCmd.Flags().StringVar(&user, "user", "", "Mining account")
	workerCmd.Flags().StringVar(&worker, "worker-name", "", "Worker name")
	group.AddCommand(workerCmd)

	var users []string
	multi := &cobra.Command{Use: "multi-account", Short: "Mining information for multiple accounts of one currency", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if currency == "" || len(users) == 0 {
				return fmt.Errorf("--currency and at least one --user are required")
			}
			for _, name := range users {
				if name == "" {
					return fmt.Errorf("--user cannot be empty")
				}
			}
			query := url.Values{"multi_account": users}
			return sendV1(cmd, opt, []string{currency, users[0]}, query)
		},
	}
	multi.Flags().StringVar(&currency, "currency", "", "Mining currency")
	multi.Flags().StringArrayVar(&users, "user", nil, "Account to query; repeat for multiple accounts")
	group.AddCommand(multi)
	return group
}

func sendV1(cmd *cobra.Command, opt *options, segments []string, query url.Values) error {
	base, err := url.Parse(opt.baseURL)
	if err != nil || base == nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" || base.Path != "" {
		return fmt.Errorf("--base-url must be an HTTP(S) origin without path or query")
	}
	if base.Scheme == "http" && base.Hostname() != "localhost" && base.Hostname() != "127.0.0.1" && base.Hostname() != "::1" {
		return fmt.Errorf("HTTP base URL is allowed only for localhost")
	}
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || strings.Contains(segment, "/") {
			return fmt.Errorf("invalid path segment %q", segment)
		}
	}
	escaped := make([]string, len(segments))
	for i, segment := range segments {
		escaped[i] = url.PathEscape(segment)
	}
	endpoint := strings.TrimRight(opt.baseURL, "/") + "/" + strings.Join(escaped, "/")
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	if opt.dryRun {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "GET %s\n", endpoint)
		return err
	}
	token := os.Getenv("F2POOL_API_TOKEN")
	if token == "" {
		return fmt.Errorf("F2POOL_API_TOKEN is required (with legacy/api_v1 permission)")
	}
	request, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("F2P-API-SECRET", token)
	request.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: opt.timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
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
