package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lihongjie0209/f2pool-cli/internal/catalog"
	"github.com/spf13/cobra"
)

type options struct {
	baseURL string
	timeout time.Duration
	output  string
	dryRun  bool
}

// Version is set from the Git tag by GoReleaser.
var Version = "dev"

func New() *cobra.Command {
	opt := &options{}
	root := &cobra.Command{Use: "f2pool", Short: "F2Pool V1 and V2 API CLI", SilenceUsage: true, SilenceErrors: true}
	root.Version = Version
	root.PersistentFlags().StringVar(&opt.baseURL, "base-url", "https://api.f2pool.com", "API origin; use a trusted HTTPS server")
	root.PersistentFlags().DurationVar(&opt.timeout, "timeout", 30*time.Second, "HTTP timeout")
	root.PersistentFlags().StringVar(&opt.output, "output", "json", "Response format: json or raw")
	root.PersistentFlags().BoolVar(&opt.dryRun, "dry-run", false, "Print the request path and JSON body without sending it")
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if opt.timeout <= 0 {
			return fmt.Errorf("--timeout must be positive")
		}
		if opt.output != "json" && opt.output != "raw" {
			return fmt.Errorf("--output must be json or raw")
		}
		return nil
	}

	endpoints, err := catalog.All()
	if err != nil {
		panic(err)
	}
	root.AddCommand(listCommand(endpoints))
	root.AddCommand(callCommand(opt))
	root.AddCommand(v1Command(opt))
	for _, endpoint := range endpoints {
		addEndpoint(root, endpoint, opt)
	}
	return root
}

func listCommand(endpoints []catalog.Endpoint) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{Use: "endpoints", Short: "List documented API endpoints and request fields", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(endpoints)
			}
			for _, ep := range endpoints {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", ep.Path, ep.Title)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print machine readable endpoint catalog")
	return cmd
}

func callCommand(opt *options) *cobra.Command {
	var data string
	cmd := &cobra.Command{Use: "call /v2/path", Short: "Call a V2 route by path, including routes added after this snapshot", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readBody(data, cmd.InOrStdin())
			if err != nil {
				return err
			}
			return send(cmd, args[0], body, opt)
		},
	}
	cmd.Flags().StringVar(&data, "data", "{}", "JSON object, @file, or @- for stdin")
	return cmd
}

func addEndpoint(root *cobra.Command, ep catalog.Endpoint, opt *options) {
	parts := strings.Split(strings.TrimPrefix(ep.Path, "/v2/"), "/")
	parent := root
	for _, part := range parts[:len(parts)-1] {
		name := strings.ReplaceAll(part, "_", "-")
		child := findChild(parent, name)
		if child == nil {
			child = &cobra.Command{Use: name, Short: "API group " + part}
			parent.AddCommand(child)
		}
		parent = child
	}
	name := strings.ReplaceAll(parts[len(parts)-1], "_", "-")
	var data string
	cmd := &cobra.Command{Use: name, Short: ep.Title, Long: ep.Title + "\nPOST " + ep.Path, Args: cobra.NoArgs}
	complex := false
	for _, field := range ep.Fields {
		if field.Type != "string" && field.Type != "bool" && field.Type != "int32" && field.Type != "int64" && field.Type != "uint32" && field.Type != "uint64" && field.Type != "float" && field.Type != "double" {
			complex = true
		}
		flag := strings.ReplaceAll(field.Name, "_", "-")
		info := field.Type
		if field.Repeated {
			info += "[]"
		}
		if field.Description != "" {
			info += ": " + field.Description
		}
		if field.Repeated {
			cmd.Flags().StringArray(flag, nil, info+"; repeat flag for each item")
		} else if field.Type == "bool" {
			cmd.Flags().Bool(flag, false, info)
		} else {
			cmd.Flags().String(flag, "", info)
		}
	}
	if complex {
		cmd.Flags().StringVar(&data, "data", "{}", "JSON object, @file, or @- for stdin; field flags override it")
	}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		body := map[string]any{}
		if complex {
			var err error
			body, err = readBody(data, cmd.InOrStdin())
			if err != nil {
				return err
			}
		}
		for _, field := range ep.Fields {
			flag := strings.ReplaceAll(field.Name, "_", "-")
			if !cmd.Flags().Changed(flag) {
				continue
			}
			if field.Repeated {
				values, _ := cmd.Flags().GetStringArray(flag)
				array := make([]any, 0, len(values))
				for _, value := range values {
					parsed, err := parseField(field.Type, value)
					if err != nil {
						return fmt.Errorf("--%s: %w", flag, err)
					}
					array = append(array, parsed)
				}
				body[field.Name] = array
			} else {
				if field.Type == "bool" {
					value, _ := cmd.Flags().GetBool(flag)
					body[field.Name] = value
				} else {
					value, _ := cmd.Flags().GetString(flag)
					parsed, err := parseField(field.Type, value)
					if err != nil {
						return fmt.Errorf("--%s: %w", flag, err)
					}
					body[field.Name] = parsed
				}
			}
		}
		return send(cmd, ep.Path, body, opt)
	}
	parent.AddCommand(cmd)
}

func findChild(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}

func readBody(spec string, stdin io.Reader) (map[string]any, error) {
	var raw []byte
	var err error
	switch {
	case spec == "@-":
		raw, err = io.ReadAll(io.LimitReader(stdin, 16<<20))
	case strings.HasPrefix(spec, "@"):
		raw, err = os.ReadFile(strings.TrimPrefix(spec, "@"))
	default:
		raw = []byte(spec)
	}
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var body map[string]any
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}
	if body == nil {
		return nil, fmt.Errorf("JSON body must be an object")
	}
	if dec.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("JSON body contains trailing data")
	}
	return body, nil
}

func parseField(kind, input string) (any, error) {
	switch kind {
	case "string":
		return input, nil
	case "bool":
		return strconv.ParseBool(input)
	case "int32", "int64", "uint32", "uint64":
		bits := 64
		if strings.HasSuffix(kind, "32") {
			bits = 32
		}
		if strings.HasPrefix(kind, "u") {
			return strconv.ParseUint(input, 10, bits)
		}
		return strconv.ParseInt(input, 10, bits)
	case "float", "double":
		return strconv.ParseFloat(input, 64)
	default:
		var value any
		dec := json.NewDecoder(strings.NewReader(input))
		dec.UseNumber()
		if err := dec.Decode(&value); err != nil {
			return nil, fmt.Errorf("expected JSON value for %s: %w", kind, err)
		}
		if dec.Decode(new(any)) != io.EOF {
			return nil, fmt.Errorf("trailing data in JSON value")
		}
		return value, nil
	}
}
