package cmd

import (
	"io"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/astronomer/astro-cli/houston"
	"github.com/astronomer/astro-cli/version"
)

var (
	workspaceId    string
	workspaceRole  string
	deploymentRole string
	role           string
	skipVerCheck   bool
	v              string
)

// NewRootCmd adds all of the primary commands for the cli
func NewRootCmd(client *houston.Client, out io.Writer) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "astro",
		Short: "Astronomer - CLI",
		Long:  "astro is a command line interface for working with the Astronomer Platform.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := setUpLogs(out, v); err != nil {
				return err
			}
			return version.ValidateCompatibility(client, out, version.CurrVersion, skipVerCheck)
		},
	}
	rootCmd.PersistentFlags().StringVarP(&v, "verbosity", "v", logrus.WarnLevel.String(), "Log level (debug, info, warn, error, fatal, panic")
	rootCmd.PersistentFlags().BoolVarP(&skipVerCheck, "skip-version-check", "", false, "skip version compatibility check")
	rootCmd.AddCommand(
		newAuthRootCmd(client, out),
		newWorkspaceCmd(client, out),
		newVersionCmd(client, out),
		newUpgradeCheckCmd(client, out),
		newUserCmd(client, out),
		newClusterRootCmd(client, out),
		newDevRootCmd(client, out),
		newCompletionCmd(client, out),
		newConfigRootCmd(client, out),
		newDeploymentRootCmd(client, out),
		newDeployCmd(client, out),
		newSaRootCmd(client, out),
		// TODO: remove newAirflowRootCmd, after 1.0 we have only devRootCmd
		newAirflowRootCmd(client, out),
		newLogsDeprecatedCmd(client, out),
	)
	return rootCmd
}

// setUpLogs set the log output ans the log level
func setUpLogs(out io.Writer, level string) error {
	logrus.SetOutput(out)
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)
	return nil
}
