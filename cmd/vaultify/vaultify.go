package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"

	"github.com/ahilsend/vaultify/pkg/leases"
	"github.com/ahilsend/vaultify/pkg/options"
	"github.com/ahilsend/vaultify/pkg/run"
	"github.com/ahilsend/vaultify/pkg/template"
)

var (
	logger = hclog.Default()

	flags = struct {
		logLevel           int
		commonOptions      options.CommonOptions
		templateOptions    template.Options
		renewLeasesOptions leases.Options
		runOptions         run.Options
	}{}

	rootCmd = &cobra.Command{
		Use:          "vaultify",
		Short:        "Vaultify templates file from vault secrets and auto renews leases",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
	}

	templateCmd = &cobra.Command{
		Use:   "template",
		Short: "Templating without renewing leases.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.templateOptions.CommonOptions = flags.commonOptions

			if !flags.templateOptions.IsValid() {
				return cmd.Help()
			}

			logger.SetLevel(logLevel())

			if err := template.Run(logger, &flags.templateOptions); err != nil {
				return fmt.Errorf("templating failed: %v", err)
			}
			fmt.Println("OK")
			return nil
		},
	}

	renewLeasesCmd = &cobra.Command{
		Use:   "renew-leases",
		Short: "Continuously renews all secret leases",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.renewLeasesOptions.CommonOptions = flags.commonOptions

			if !flags.renewLeasesOptions.IsValid() {
				return cmd.Help()
			}

			logger.SetLevel(logLevel())

			if err := leases.Run(logger, &flags.renewLeasesOptions); err != nil {
				return fmt.Errorf("renew-leases failed: %v", err)
			}
			return nil
		},
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Templates a configuration file, and then continuously renews the secret leases. This is combines `template` and `renew-leases`, and does not require writing the lease information to file.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.runOptions.CommonOptions = flags.commonOptions

			if !flags.runOptions.IsValid() {
				return cmd.Help()
			}

			logger.SetLevel(logLevel())

			if err := run.Run(logger, &flags.runOptions); err != nil {
				return fmt.Errorf("run failed: %v", err)
			}
			return nil
		},
	}
)

func logLevel() hclog.Level {
	switch flags.logLevel {
	case 0:
		return hclog.Error
	case 1:
		return hclog.Warn
	case 2:
		return hclog.Info
	case 3:
		return hclog.Debug
	}
	return hclog.Trace
}

func init() {
	rootCmd.PersistentFlags().CountVarP(
		&flags.logLevel,
		"verbose",
		"v",
		"Log level. Defaults to 'error', Set multiple times to increase log level")
	rootCmd.PersistentFlags().StringVar(
		&flags.commonOptions.VaultAddress,
		"vault",
		"",
		"Vault address. Can be specified via VAULT_ADDR instead")
	rootCmd.PersistentFlags().DurationVar(
		&flags.commonOptions.Timeout,
		"timeout",
		time.Minute,
		"Timeout for requests. Can be specified via VAULT_CLIENT_TIMEOUT instead")
	rootCmd.PersistentFlags().IntVar(
		&flags.commonOptions.MaxRetries,
		"max-retries",
		20,
		"Max retries for requests. Can be specified via VAULT_MAX_RETRIES instead")
	rootCmd.PersistentFlags().IntVar(
		&flags.commonOptions.RateLimitBurst,
		"rate-limit-burst",
		2,
		"Rate limiting of requests, perform max X bursts")
	rootCmd.PersistentFlags().DurationVar(
		&flags.commonOptions.RateLimit,
		"rate-limit",
		30*time.Second,
		"Rate limiting of requests, perform burst every X duration")

	flag.CommandLine.VisitAll(func(gf *flag.Flag) {
		rootCmd.PersistentFlags().AddGoFlag(gf)
	})

	templateCmd.Flags().StringVar(&flags.templateOptions.Role, "role", "", "Vault kubernetes role to assume")
	templateCmd.Flags().StringVar(&flags.templateOptions.TemplateFileName, "template-file", "", "Template file to render")
	templateCmd.Flags().StringVar(&flags.templateOptions.OutputFileName, "output-file", "", "Output file")
	templateCmd.Flags().StringVar(&flags.templateOptions.SecretsOutputFileName, "secrets-output-file", "", "Secrets output file")
	templateCmd.Flags().StringToStringVar(&flags.templateOptions.Variables, "var", map[string]string{}, "Variables to use instead of fetching secrets from vault. Does not require vault, this is for testing the templating only.")

	renewLeasesCmd.Flags().StringVar(&flags.renewLeasesOptions.SecretsFileName, "secrets-file", "", "Secrets file")
	renewLeasesCmd.Flags().StringVar(&flags.renewLeasesOptions.MetricsAddress, "metrics-address", ":9105", "Metrics address")
	renewLeasesCmd.Flags().StringVar(&flags.renewLeasesOptions.MetricsPath, "metrics-path", "/metrics", "Metrics path")

	runCmd.Flags().StringVar(&flags.runOptions.Role, "role", "", "Vault kubernetes role to assume")
	runCmd.Flags().StringVar(&flags.runOptions.TemplateFileName, "template-file", "", "Template file to render")
	runCmd.Flags().StringVar(&flags.runOptions.OutputFileName, "output-file", "", "Output file")
	runCmd.Flags().StringVar(&flags.runOptions.MetricsAddress, "metrics-address", ":9105", "Metrics address")
	runCmd.Flags().StringVar(&flags.runOptions.MetricsPath, "metrics-path", "/metrics", "Metrics path")

	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(renewLeasesCmd)
	rootCmd.AddCommand(runCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}
