package cmd

import (
	"encoding/json"
	"io"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/converter/har"
	"go.k6.io/k6/lib"
)

// TODO: split apart like `k6 run` and `k6 archive`?
//nolint:funlen,gocognit
func getCmdConvert(globalState *globalState) *cobra.Command {
	var (
		convertOutput       string
		optionsFilePath     string
		minSleep            uint
		maxSleep            uint
		enableChecks        bool
		returnOnFailedCheck bool
		correlate           bool
		threshold           uint
		nobatch             bool
		only                []string
		skip                []string
	)
	convertCmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert a HAR file to a k6 script",
		Long:  "Convert a HAR (HTTP Archive) file to a k6 script",
		Example: `
  # Convert a HAR file to a k6 script.
  k6 convert -O har-session.js session.har

  # Convert a HAR file to a k6 script creating requests only for the given domain/s.
  k6 convert -O har-session.js --only yourdomain.com,additionaldomain.com session.har

  # Convert a HAR file. Batching requests together as long as idle time between requests <800ms
  k6 convert --batch-threshold 800 session.har

  # Run the k6 script.
  k6 run har-session.js`[1:],
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse the HAR file
			r, err := globalState.fs.Open(args[0])
			if err != nil {
				return err
			}
			h, err := har.Decode(r)
			if err != nil {
				return err
			}
			if err = r.Close(); err != nil {
				return err
			}

			// recordings include redirections as separate requests, and we dont want to trigger them twice
			options := lib.Options{MaxRedirects: null.IntFrom(0)}

			if optionsFilePath != "" {
				optionsFileContents, readErr := afero.ReadFile(globalState.fs, optionsFilePath)
				if readErr != nil {
					return readErr
				}
				var injectedOptions lib.Options
				if err := json.Unmarshal(optionsFileContents, &injectedOptions); err != nil {
					return err
				}
				options = options.Apply(injectedOptions)
			}

			// TODO: refactor...
			script, err := har.Convert(h, options, minSleep, maxSleep, enableChecks,
				returnOnFailedCheck, threshold, nobatch, correlate, only, skip)
			if err != nil {
				return err
			}

			// Write script content to stdout or file
			if convertOutput == "" || convertOutput == "-" { //nolint:nestif
				if _, err := io.WriteString(globalState.stdOut, script); err != nil {
					return err
				}
			} else {
				f, err := globalState.fs.Create(convertOutput)
				if err != nil {
					return err
				}
				if _, err := f.WriteString(script); err != nil {
					return err
				}
				if err := f.Sync(); err != nil {
					return err
				}
				if err := f.Close(); err != nil {
					return err
				}
			}
			return nil
		},
	}

	convertCmd.Flags().SortFlags = false
	convertCmd.Flags().StringVarP(
		&convertOutput, "output", "O", convertOutput,
		"k6 script output filename (stdout by default)",
	)
	convertCmd.Flags().StringVarP(
		&optionsFilePath, "options", "", optionsFilePath,
		"path to a JSON file with options that would be injected in the output script",
	)
	convertCmd.Flags().StringSliceVarP(&only, "only", "", []string{}, "include only requests from the given domains")
	convertCmd.Flags().StringSliceVarP(&skip, "skip", "", []string{}, "skip requests from the given domains")
	convertCmd.Flags().UintVarP(&threshold, "batch-threshold", "", 500, "batch request idle time threshold (see example)")
	convertCmd.Flags().BoolVarP(&nobatch, "no-batch", "", false, "don't generate batch calls")
	convertCmd.Flags().BoolVarP(&enableChecks, "enable-status-code-checks", "", false, "add a status code check for each HTTP response")                                                                          //nolint:lll
	convertCmd.Flags().BoolVarP(&returnOnFailedCheck, "return-on-failed-check", "", false, "return from iteration if we get an unexpected response status code")                                                  //nolint:lll
	convertCmd.Flags().BoolVarP(&correlate, "correlate", "", false, "detect values in responses being used in subsequent requests and try adapt the script accordingly (only redirects and JSON values for now)") //nolint:lll
	convertCmd.Flags().UintVarP(&minSleep, "min-sleep", "", 20, "the minimum amount of seconds to sleep after each iteration")                                                                                    //nolint:lll
	convertCmd.Flags().UintVarP(&maxSleep, "max-sleep", "", 40, "the maximum amount of seconds to sleep after each iteration")                                                                                    //nolint:lll
	return convertCmd
}
