package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
)

const cloudPlanCommandName = "plan"

type cmdCloudPlan struct {
	gs         *state.GlobalState
	outputJSON bool
}

func (c *cmdCloudPlan) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))
	flags.BoolVar(&c.outputJSON, "json", false, "Output result in JSON format")
	return flags
}

func getCmdCloudPlan(gs *state.GlobalState) *cobra.Command {
	c := &cmdCloudPlan{
		gs: gs,
	}

	exampleText := getExampleText(gs, `
  # Display the current Grafana Cloud k6 plan
  $ {{.}} cloud plan script.js`[1:])

	planCloudCommand := &cobra.Command{
		Use:     cloudPlanCommandName + " [script]",
		Short:   "Display the current Grafana Cloud k6 plan",
		Long:    `Display the current Grafana Cloud k6 plan.`,
		Example: exampleText,
		Args:    cobra.ExactArgs(1),
		RunE:    c.run,
	}

	planCloudCommand.Flags().SortFlags = false
	planCloudCommand.Flags().AddFlagSet(c.flagSet())

	return planCloudCommand
}

func (c *cmdCloudPlan) run(cmd *cobra.Command, args []string) error {
	test, err := loadAndConfigureLocalTest(c.gs, cmd, args, getPartialConfig)
	if err != nil {
		return err
	}

	testRunState, err := test.buildTestRunState(test.consolidatedConfig.Options)
	if err != nil {
		return err
	}

	arc := testRunState.Runner.MakeArchive()

	cloudConfig, warn, err := cloudapi.GetConsolidatedConfig(
		test.derivedConfig.Collectors["cloud"], c.gs.Env, "", arc.Options.Cloud, arc.Options.External)
	if err != nil {
		return err
	}
	if !cloudConfig.Token.Valid {
		return errors.New( //nolint:golint
			"not logged in, please login first to the Grafana Cloud k6 " +
				"using the \"k6 cloud login\" command",
		)
	}

	if warn != "" {
		fmt.Fprintf(c.gs.Stderr, "Warning: %s\n", warn)
	}

	if !cloudConfig.Token.Valid {
		return errors.New("missing a valid Grafana Cloud k6 token")
	}

	if !cloudConfig.ProjectID.Valid {
		return errors.New("missing a valid Grafana Cloud k6 ProjectID (set it in the script or via the K6_CLOUD_PROJECT_ID environment variable)")
	}

	cloudContext, err := getCloudContext(cloudConfig.Token.String, int(cloudConfig.ProjectID.Int64))
	if err != nil {
		return fmt.Errorf("error finding stack: %w", err)
	}

	optionsMap, _ := StructToMap(cloudConfig)
	validationResponse, err := validateOptions(cloudConfig.Token.String, cloudContext.StackID, cloudContext.ProjectID, optionsMap)
	if err != nil {
		return fmt.Errorf("error validating options: %w", err)
	}

	if c.outputJSON {
		output := map[string]interface{}{
			"stack": map[string]interface{}{
				"name": cloudContext.StackName,
				"id":   cloudContext.StackID,
			},
			"project": map[string]interface{}{
				"name": cloudContext.ProjectName,
				"id":   cloudContext.ProjectID,
			},
			"estimated_vuh_usage": validationResponse.VuhUsage,
		}
		jsonOutput, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshalling JSON output: %w", err)
		}
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Printf("Stack: %s (%d)\nProject: %s (%d)\n", cloudContext.StackName, cloudContext.StackID, cloudContext.ProjectName, cloudContext.ProjectID)
		fmt.Println(fmt.Sprintf("Estimated Usage: %v VUh  ", validationResponse.VuhUsage))
	}

	return nil
}

type ValidationResponse struct {
	VuhUsage float64 `json:"vuh_usage"`
}

func validateOptions(token string, stackID int, projectID int, options map[string]interface{}) (ValidationResponse, error) {
	url := "https://api.k6.io/cloud/v6/validate_options"

	reqBody, err := json.Marshal(map[string]interface{}{
		"project_id": projectID,
		"options":    options,
	})
	if err != nil {
		return ValidationResponse{}, fmt.Errorf("error marshalling request body: %w", err)
	}

	req, err := http.NewRequest("POST", url, io.NopCloser(bytes.NewReader(reqBody)))
	if err != nil {
		return ValidationResponse{}, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Stack-Id", fmt.Sprintf("%d", stackID))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ValidationResponse{}, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return ValidationResponse{}, fmt.Errorf("error reading response body: %w", err)
		}
		type ValidationError struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		}
		type ValidateOptionsResponseFailed struct {
			Error ValidationError `json:"error"`
		}
		validationError := ValidateOptionsResponseFailed{}
		err = json.Unmarshal(body, &validationError)
		if err != nil {
			return ValidationResponse{}, fmt.Errorf("error unmarshalling response body: %w", err)
		}
		return ValidationResponse{}, fmt.Errorf("error validating options: %s", validationError.Error.Message)
	}

	var validationResponse ValidationResponse
	err = json.NewDecoder(resp.Body).Decode(&validationResponse)
	if err != nil {
		return ValidationResponse{}, fmt.Errorf("error decoding response body: %w", err)
	}

	return validationResponse, nil
}

type CloudContext struct {
	StackID     int
	StackName   string
	ProjectID   int
	ProjectName string
}

func getCloudContext(token string, projectID int) (CloudContext, error) {
	type Organization struct {
		ID               int    `json:"id"`
		GrafanaStackName string `json:"grafana_stack_name"`
		GrafanaStackID   int    `json:"grafana_stack_id"`
	}

	type MeResponse struct {
		Organizations []Organization `json:"organizations"`
	}

	type ProjectResponse struct {
		Name string `json:"name"`
	}

	url := "https://api.k6.io/v3/account/me"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return CloudContext{}, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "token "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return CloudContext{}, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CloudContext{}, fmt.Errorf("error reading response body: %w", err)
	}

	var meres MeResponse
	if err := json.Unmarshal(body, &meres); err != nil {
		return CloudContext{}, fmt.Errorf("error unmarshalling response: %w", err)
	}

	for _, org := range meres.Organizations {
		if org.GrafanaStackID != 0 {
			url2 := fmt.Sprintf("https://api.k6.io/cloud/v6/projects/%d", projectID)
			req2, err := http.NewRequest("GET", url2, nil)
			if err != nil {
				return CloudContext{}, fmt.Errorf("error creating request: %w", err)
			}

			req2.Header.Set("Authorization", "Bearer "+token)
			req2.Header.Set("X-Stack-Id", fmt.Sprintf("%d", org.GrafanaStackID))

			resp2, err := client.Do(req2)
			if err != nil {
				return CloudContext{}, fmt.Errorf("error making request: %w", err)
			}
			defer resp2.Body.Close()

			if resp2.StatusCode == 200 {
				body2, err := io.ReadAll(resp2.Body)
				if err != nil {
					return CloudContext{}, fmt.Errorf("error reading response body: %w", err)
				}

				var pres ProjectResponse
				if err := json.Unmarshal(body2, &pres); err != nil {
					return CloudContext{}, fmt.Errorf("error unmarshalling response: %w", err)
				}

				return CloudContext{
					StackID:     org.GrafanaStackID,
					StackName:   org.GrafanaStackName,
					ProjectID:   projectID,
					ProjectName: pres.Name,
				}, nil
			}
		}
	}

	return CloudContext{}, fmt.Errorf("no stack found for project ID %d", projectID)
}

func StructToMap(s interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
