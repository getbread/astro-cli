/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"io"

	"github.com/astronomer/astro-cli/daggenerate"
	"github.com/astronomer/astro-cli/houston"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	name        string
	source      string
	destination string
	dryrun      bool
)

// dagCmd represents the dag command
func daggenerateCmd(client *houston.Client, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dag-generate",
		Short: "Generate a DAG that follows Airflow best practices",
		Long: `Generate a DAG that follows Airflow best practices

  Valid sources: google-analytics

  Valid destinations: snowflake`,
		Aliases: []string{"dg"},
		// ignore PersistentPreRunE of root command
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDag(client, cmd, out, args)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if source != "google-analytics" {
				return errors.New("Invalid source")
			}
			if destination != "snowflake" {
				return errors.New("Invalid destination")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "DAG (and file) name")
	cmd.MarkFlagRequired("name")
	cmd.Flags().StringVar(&source, "source", "", "DAG source")
	cmd.MarkFlagRequired("source")
	cmd.Flags().StringVar(&destination, "destination", "", "DAG destination")
	cmd.MarkFlagRequired("destination")
	cmd.Flags().BoolVar(&dryrun, "dryrun", false, "Dry run")
	return cmd
}

func printDag(client *houston.Client, cmd *cobra.Command, out io.Writer, args []string) error {
	// Silence Usage as we have now validated command input
	cmd.SilenceUsage = true

	err := daggenerate.Generate(source, destination, name, dryrun, out)
	if err != nil {
		return err
	}
	return nil
}
