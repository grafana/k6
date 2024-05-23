// SPDX-FileCopyrightText: 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

package dashboard

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
)

const (
	flagHost   = "host"
	flagPort   = "port"
	flagPeriod = "period"
	flagOpen   = "open"
	flagExport = "export"
	flagTags   = "tags"
)

// NewCommand build dashboard sub-command.
func NewCommand(gs *state.GlobalState) *cobra.Command {
	proc := new(process).fromGlobalState(gs)
	assets := newCustomizedAssets(proc)

	dashboardCmd := &cobra.Command{ //nolint:exhaustruct
		Use:   OutputName,
		Short: "Offline k6 web dashboard management",
		Long:  `k6 web dashboard management that does not require running k6 (recording playback, creating a report from a recording, etc.).`, //nolint:lll
	}

	dashboardCmd.AddCommand(newReplayCommand(assets, proc))
	dashboardCmd.AddCommand(newAggregateCommand(proc))
	dashboardCmd.AddCommand(newReportCommand(assets, proc))

	return dashboardCmd
}

func newReplayCommand(assets *assets, proc *process) *cobra.Command {
	opts := new(options)

	cmd := &cobra.Command{ //nolint:exhaustruct
		Use:   "replay file",
		Short: "Load the recorded dashboard events and replay it for the UI",
		Long: `The replay command load the recorded dashboard events (NDJSON format) and replay it for the dashboard UI.
The compressed file will be automatically decompressed if the file extension is .gz`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			err := replay(args[0], opts, assets, proc)
			if err != nil {
				return err
			}

			if opts.Port < 0 {
				return nil
			}

			done := make(chan os.Signal, 1)

			signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

			<-done

			return nil
		},
	}

	cmd.Flags().SortFlags = false

	flags := cmd.PersistentFlags()

	flags.StringVar(
		&opts.Host,
		flagHost,
		defaultHost,
		"Hostname or IP address for HTTP endpoint (default: '', empty, listen on all interfaces)",
	)
	flags.IntVar(
		&opts.Port,
		flagPort,
		defaultPort,
		"TCP port for HTTP endpoint (0=random, -1=no HTTP), example: 8080",
	)
	flags.BoolVar(&opts.Open, flagOpen, defaultOpen, "Open browser window automatically")
	flags.StringVar(
		&opts.Export,
		flagExport,
		defaultExport,
		"Report file location (default: '', no report)",
	)

	return cmd
}

func newAggregateCommand(proc *process) *cobra.Command {
	opts := new(options)
	cmd := &cobra.Command{ //nolint:exhaustruct
		Use:   "aggregate input-file output-file",
		Short: "Convert saved json output to recorded dashboard events",
		Long: `The aggregate command converts the file saved by json output to dashboard format events file.
The files will be automatically compressed/decompressed if the file extension is .gz`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return aggregate(args[0], args[1], opts, proc)
		},
	}

	cmd.Flags().SortFlags = false

	flags := cmd.PersistentFlags()

	flags.DurationVar(
		&opts.Period,
		flagPeriod,
		defaultPeriod,
		"Event emitting frequency, example: `1m`",
	)
	flags.StringSliceVar(
		&opts.Tags,
		flagTags,
		defaultTags(),
		"Precomputed metric tags, can be specified more than once",
	)

	return cmd
}

func newReportCommand(assets *assets, proc *process) *cobra.Command {
	opts := new(options)

	cmd := &cobra.Command{ //nolint:exhaustruct
		Use:   "report events-file report-file",
		Short: "Create report from a recorded event file",
		Long: `The report command loads recorded dashboard events (NDJSON format) and creates a report.
The compressed events file will be automatically decompressed if the file extension is .gz`,
		Example: `# Visualize the result of a previous test run (using events file):
$ k6 run --` + OutputName + `=record=test_result.ndjson script.js
$ k6 ` + OutputName + ` replay test_result.ndjson

# Visualize the result of a previous test run (using json output):
$ k6 run --out json=test_result.json script.js
$ k6 ` + OutputName + ` aggregate test_result.json test_result.ndjson
$ k6 ` + OutputName + ` replay test_result.ndjson

# Generate report from previous test run (using events file):
$ k6 run --out web-dashboard=record=test_result.ndjson script.js
$ k6 ` + OutputName + ` report test_result.ndjson test_result_report.html`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			opts.Port = -1
			opts.Export = args[1]

			if err := replay(args[0], opts, assets, proc); err != nil {
				return err
			}

			if opts.Open {
				_ = browser.OpenFile(args[1])
			}

			return nil
		},
	}

	cmd.Flags().SortFlags = false

	flags := cmd.PersistentFlags()

	flags.BoolVar(&opts.Open, flagOpen, defaultOpen, "Open browser window with generated report")

	return cmd
}
