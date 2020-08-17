/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats/cloud"
	"github.com/loadimpact/k6/ui"
	"github.com/loadimpact/k6/ui/pb"
)

const (
	cloudFailedToGetProgressErrorCode = 98
	cloudTestRunFailedErrorCode       = 99
)

//nolint:gochecknoglobals
var exitOnRunning = os.Getenv("K6_EXIT_ON_RUNNING") != ""

//nolint:gochecknoglobals
var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Run a test on the cloud",
	Long: `Run a test on the cloud.

This will execute the test on the Load Impact cloud service. Use "k6 login cloud" to authenticate.`,
	Example: `
        k6 cloud script.js`[1:],
	Args: exactArgsWithMsg(1, "arg should either be \"-\", if reading script from stdin, or a path to a script file"),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: disable in quiet mode?
		_, _ = BannerColor.Fprintf(stdout, "\n%s\n\n", consts.Banner)

		progressBar := pb.New(
			pb.WithConstLeft(" Init"),
			pb.WithConstProgress(0, "Parsing script"),
		)
		printBar(progressBar)

		// Runner
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}

		filename := args[0]
		filesystems := loader.CreateFilesystems()
		// TODO: don't use the Global logger
		logger := logrus.StandardLogger()
		src, err := loader.ReadSource(logger, filename, pwd, filesystems, os.Stdin)
		if err != nil {
			return err
		}

		runtimeOptions, err := getRuntimeOptions(cmd.Flags(), buildEnvMap(os.Environ()))
		if err != nil {
			return err
		}

		modifyAndPrintBar(progressBar, pb.WithConstProgress(0, "Getting script options"))
		r, err := newRunner(logger, src, runType, filesystems, runtimeOptions)
		if err != nil {
			return err
		}

		modifyAndPrintBar(progressBar, pb.WithConstProgress(0, "Consolidating options"))
		cliOpts, err := getOptions(cmd.Flags())
		if err != nil {
			return err
		}
		conf, err := getConsolidatedConfig(afero.NewOsFs(), Config{Options: cliOpts}, r)
		if err != nil {
			return err
		}

		derivedConf, cerr := deriveAndValidateConfig(conf, r.IsExecutable)
		if cerr != nil {
			return ExitCode{error: cerr, Code: invalidConfigErrorCode}
		}

		// TODO: validate for usage of execution segment
		// TODO: validate for externally controlled executor (i.e. executors that aren't distributable)
		// TODO: move those validations to a separate function and reuse validateConfig()?

		err = r.SetOptions(conf.Options)
		if err != nil {
			return err
		}

		// Cloud config
		cloudConfig := cloud.NewConfig().Apply(derivedConf.Collectors.Cloud)
		if err = envconfig.Process("", &cloudConfig); err != nil {
			return err
		}
		if !cloudConfig.Token.Valid {
			return errors.New("Not logged in, please use `k6 login cloud`.")
		}

		modifyAndPrintBar(progressBar, pb.WithConstProgress(0, "Building the archive"))
		arc := r.MakeArchive()
		// TODO: Fix this
		// We reuse cloud.Config for parsing options.ext.loadimpact, but this probably shouldn't be
		// done as the idea of options.ext is that they are extensible without touching k6. But in
		// order for this to happen we shouldn't actually marshall cloud.Config on top of it because
		// it will be missing some fields that aren't actually mentioned in the struct.
		// So in order for use to copy the fields that we need for loadimpact's api we unmarshal in
		// map[string]interface{} and copy what we need if it isn't set already
		var tmpCloudConfig map[string]interface{}
		if val, ok := arc.Options.External["loadimpact"]; ok {
			dec := json.NewDecoder(bytes.NewReader(val))
			dec.UseNumber() // otherwise float64 are used
			if err := dec.Decode(&tmpCloudConfig); err != nil {
				return err
			}
		}

		if err := cloud.MergeFromExternal(arc.Options.External, &cloudConfig); err != nil {
			return err
		}
		if tmpCloudConfig == nil {
			tmpCloudConfig = make(map[string]interface{}, 3)
		}

		if _, ok := tmpCloudConfig["token"]; !ok && cloudConfig.Token.Valid {
			tmpCloudConfig["token"] = cloudConfig.Token
		}
		if _, ok := tmpCloudConfig["name"]; !ok && cloudConfig.Name.Valid {
			tmpCloudConfig["name"] = cloudConfig.Name
		}
		if _, ok := tmpCloudConfig["projectID"]; !ok && cloudConfig.ProjectID.Valid {
			tmpCloudConfig["projectID"] = cloudConfig.ProjectID
		}

		if arc.Options.External == nil {
			arc.Options.External = make(map[string]json.RawMessage)
		}
		arc.Options.External["loadimpact"], err = json.Marshal(tmpCloudConfig)
		if err != nil {
			return err
		}

		name := cloudConfig.Name.String
		if !cloudConfig.Name.Valid || cloudConfig.Name.String == "" {
			name = filepath.Base(filename)
		}

		// Start cloud test run
		modifyAndPrintBar(progressBar, pb.WithConstProgress(0, "Validating script options"))
		client := cloud.NewClient(logger, cloudConfig.Token.String, cloudConfig.Host.String, consts.Version)
		if err := client.ValidateOptions(arc.Options); err != nil {
			return err
		}

		modifyAndPrintBar(progressBar, pb.WithConstProgress(0, "Uploading archive"))
		refID, err := client.StartCloudTestRun(name, cloudConfig.ProjectID.Int64, arc)
		if err != nil {
			return err
		}

		et, err := lib.NewExecutionTuple(derivedConf.ExecutionSegment, derivedConf.ExecutionSegmentSequence)
		if err != nil {
			return err
		}
		testURL := cloud.URLForResults(refID, cloudConfig)
		executionPlan := derivedConf.Scenarios.GetFullExecutionRequirements(et)
		printExecutionDescription("cloud", filename, testURL, derivedConf, et, executionPlan, nil)

		modifyAndPrintBar(
			progressBar,
			pb.WithConstLeft(" Run "),
			pb.WithConstProgress(0, "Initializing the cloud test"),
		)

		// The quiet option hides the progress bar and disallow aborting the test
		if quiet {
			return nil
		}

		// Trap Interrupts, SIGINTs and SIGTERMs.
		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigC)

		var (
			startTime   time.Time
			maxDuration time.Duration
		)
		maxDuration, _ = lib.GetEndOffset(executionPlan)

		testProgress := &cloud.TestProgressResponse{}
		progressBar.Modify(
			pb.WithProgress(func() (float64, []string) {
				statusText := testProgress.RunStatusText

				if testProgress.RunStatus == lib.RunStatusRunning {
					if startTime.IsZero() {
						startTime = time.Now()
					}
					spent := time.Since(startTime)
					if spent > maxDuration {
						statusText = maxDuration.String()
					} else {
						statusText = fmt.Sprintf("%s/%s", pb.GetFixedLengthDuration(spent, maxDuration), maxDuration)
					}
				}

				return testProgress.Progress, []string{statusText}
			}),
		)

		var progressErr error
		ticker := time.NewTicker(time.Millisecond * 2000)
		shouldExitLoop := false

	runningLoop:
		for {
			select {
			case <-ticker.C:
				testProgress, progressErr = client.GetTestProgress(refID)
				if progressErr == nil {
					if (testProgress.RunStatus > lib.RunStatusRunning) || (exitOnRunning && testProgress.RunStatus == lib.RunStatusRunning) {
						shouldExitLoop = true
					}
					printBar(progressBar)
				} else {
					logger.WithError(progressErr).Error("Test progress error")
				}
				if shouldExitLoop {
					break runningLoop
				}
			case sig := <-sigC:
				logger.WithField("sig", sig).Print("Exiting in response to signal...")
				err := client.StopCloudTestRun(refID)
				if err != nil {
					logger.WithError(err).Error("Stop cloud test error")
				}
				shouldExitLoop = true // Exit after the next GetTestProgress call
			}
		}

		if testProgress == nil {
			//nolint:golint
			return ExitCode{error: errors.New("Test progress error"), Code: cloudFailedToGetProgressErrorCode}
		}

		fprintf(stdout, "     test status: %s\n", ui.ValueColor.Sprint(testProgress.RunStatusText))

		if testProgress.ResultStatus == cloud.ResultStatusFailed {
			//nolint:golint
			return ExitCode{error: errors.New("The test has failed"), Code: cloudTestRunFailedErrorCode}
		}

		return nil
	},
}

func cloudCmdFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))

	// TODO: Figure out a better way to handle the CLI flags:
	// - the default value is specified in this way so we don't overwrire whatever
	//   was specified via the environment variable
	// - global variables are not very testable... :/
	flags.BoolVar(&exitOnRunning, "exit-on-running", exitOnRunning, "exits when test reaches the running status")
	// We also need to explicitly set the default value for the usage message here, so setting
	// K6_EXIT_ON_RUNNING=true won't affect the usage message
	flags.Lookup("exit-on-running").DefValue = "false"

	return flags
}

func init() {
	RootCmd.AddCommand(cloudCmd)
	cloudCmd.Flags().SortFlags = false
	cloudCmd.Flags().AddFlagSet(cloudCmdFlagSet())
}
