// Command mflint lints Kubernetes manifests for security/reliability
// issues and estimates their monthly cloud cost, via the mflint API so
// checks count against the account's plan the same way the web app does.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	format      string
	cloud       string
	noCost      bool
	apiBaseFlag string
	tokenFlag   string
)

func main() {
	root := &cobra.Command{
		Use:   "mflint [path]",
		Short: "Lint Kubernetes manifests and estimate their monthly cloud cost",
		Long: "mflint parses Kubernetes manifests (a single file or a directory), checks them\n" +
			"against a fixed set of security/reliability rules, and estimates monthly\n" +
			"cloud cost from container resource requests/limits. Cost figures are static\n" +
			"list-price estimates, not a guaranteed bill.\n\n" +
			"Every check runs against the mflint API. Anonymous runs are IP rate-limited;\n" +
			"run `mflint login` to use your account's plan and monthly quota instead.",
		Args:          cobra.MaximumNArgs(1),
		RunE:          runLint,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.Flags().StringVar(&format, "format", "table", "output format: table or json")
	root.Flags().StringVar(&cloud, "cloud", "aws", "cloud for cost estimation: aws, gcp, or azure")
	root.Flags().BoolVar(&noCost, "no-cost", false, "skip cost estimation, lint only")
	root.PersistentFlags().StringVar(&apiBaseFlag, "api-base", "", "mflint API base URL (overrides MFLINT_API_BASE and saved config)")
	root.PersistentFlags().StringVar(&tokenFlag, "token", "", "API key for this run (overrides MFLINT_API_KEY and saved config)")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the mflint version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("mflint dev")
		},
	})
	root.AddCommand(newLoginCmd())
	root.AddCommand(newLogoutCmd())
	root.AddCommand(newWhoamiCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}

func resolveAPIBase(cfg Config) string {
	if apiBaseFlag != "" {
		return apiBaseFlag
	}
	if v := os.Getenv("MFLINT_API_BASE"); v != "" {
		return v
	}
	if cfg.APIBase != "" {
		return cfg.APIBase
	}
	return defaultAPIBase
}

func resolveToken(cfg Config) string {
	if tokenFlag != "" {
		return tokenFlag
	}
	if v := os.Getenv("MFLINT_API_KEY"); v != "" {
		return v
	}
	return cfg.Token
}

func runLint(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) == 1 {
		path = args[0]
	}
	if format != "table" && format != "json" {
		return fmt.Errorf("invalid --format %q: must be table or json", format)
	}

	manifest, err := readManifest(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(manifest) == "" {
		fmt.Fprintf(os.Stderr, "warning: no .yaml/.yml content found in %s\n", path)
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	apiBase := resolveAPIBase(cfg)
	token := resolveToken(cfg)

	rep, err := lintViaAPI(cmd.Context(), apiBase, token, manifest, cloud, noCost)
	if err != nil {
		return err
	}

	var writeErr error
	if format == "json" {
		writeErr = rep.WriteJSON(os.Stdout)
	} else {
		writeErr = rep.WriteTable(os.Stdout)
	}
	if writeErr != nil {
		return writeErr
	}

	if token == "" {
		fmt.Fprintf(os.Stderr, "\nTip: run `mflint login` for a stable monthly quota instead of IP rate limiting — sign up free at %s/login\n", apiBase)
	}

	if rep.HasCritical() {
		os.Exit(1)
	}
	return nil
}

// readManifest reads path (a file) or joins every .yaml/.yml file under it
// (a directory, sorted for determinism) into one multi-document manifest,
// since the API accepts a single YAML string per request.
func readManifest(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if !info.IsDir() {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("%s: %w", path, err)
		}
		return string(b), nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking %s: %w", path, err)
	}
	sort.Strings(files)

	var parts []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("%s: %w", f, err)
		}
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n---\n"), nil
}
