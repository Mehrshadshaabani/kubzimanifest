package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Save an mflint API key so lint runs count toward your account's plan",
		Long: "Generate an API key at <api-base>/app -> \"API keys\" -> Generate new key, then\n" +
			"paste it here. It's saved locally and sent as a bearer token on every future\n" +
			"`mflint` run, so checks count against your plan's monthly quota instead of the\n" +
			"unauthenticated, IP rate-limited path.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			apiBase := resolveAPIBase(cfg)

			token := tokenFlag
			if token == "" {
				fmt.Printf("Paste your mflint API key (from %s/app -> API keys): ", apiBase)
				line, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return fmt.Errorf("reading API key: %w", err)
				}
				token = strings.TrimSpace(line)
			}
			if token == "" {
				return fmt.Errorf("no API key provided")
			}

			usage, err := fetchUsage(cmd.Context(), apiBase, token)
			if err != nil {
				return fmt.Errorf("could not verify API key: %w", err)
			}

			cfg.APIBase = apiBase
			cfg.Token = token
			if err := saveConfig(cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Logged in against %s\n", apiBase)
			printUsage(usage)
			return nil
		},
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the saved API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			cfg.Token = ""
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Println("Logged out. Future runs are anonymous (IP rate-limited, no saved monthly quota).")
			return nil
		},
	}
}

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the current plan and this month's usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			apiBase := resolveAPIBase(cfg)
			token := resolveToken(cfg)
			if token == "" {
				fmt.Println("Not logged in. Run `mflint login` to check a saved plan/quota; without it, checks run anonymously (IP rate-limited, no monthly quota).")
				return nil
			}
			usage, err := fetchUsage(cmd.Context(), apiBase, token)
			if err != nil {
				return fmt.Errorf("fetching usage from %s: %w", apiBase, err)
			}
			printUsage(usage)
			return nil
		},
	}
}

func printUsage(u *usageResponse) {
	if u.Limit <= 0 {
		fmt.Printf("Plan: %s — %d checks used this month (unlimited)\n", u.Plan, u.Used)
		return
	}
	fmt.Printf("Plan: %s — %d/%d checks used this month (%d remaining)\n", u.Plan, u.Used, u.Limit, u.Remaining)
}
