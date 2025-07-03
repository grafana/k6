package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
)

// PolicyConfig represents the structure of k6policy.json
type PolicyConfig struct {
	RequireThresholds bool     `json:"requireThresholds"`
	RequiredTags      []string `json:"requiredTags"`
	DisallowedStrings []string `json:"disallowedStrings"`
	DisallowedRegex   []string `json:"disallowedRegex"`
}

// PolicyViolation represents a single policy violation
type PolicyViolation struct {
	Type        string
	Description string
}

// PolicyResult contains the results of policy validation
type PolicyResult struct {
	PolicyFile string
	Violations []PolicyViolation
	Used       bool
}

// PolicyChecker handles policy validation logic
type PolicyChecker struct {
	fs fsext.Fs
}

// NewPolicyChecker creates a new PolicyChecker
func NewPolicyChecker(fs fsext.Fs) *PolicyChecker {
	return &PolicyChecker{fs: fs}
}

// LoadPolicy loads a policy configuration from a file
func (pc *PolicyChecker) LoadPolicy(policyPath string) (*PolicyConfig, error) {
	data, err := fsext.ReadFile(pc.fs, policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file %s: %w", policyPath, err)
	}

	var policy PolicyConfig
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy file %s: %w", policyPath, err)
	}

	return &policy, nil
}

// FindPolicyFile looks for k6policy.json in the same directory as the script
func (pc *PolicyChecker) FindPolicyFile(scriptPath string) (string, bool) {
	dir := filepath.Dir(scriptPath)
	policyPath := filepath.Join(dir, "k6policy.json")

	exists, err := fsext.Exists(pc.fs, policyPath)
	if err != nil || !exists {
		return "", false
	}

	return policyPath, true
}

// ValidatePolicy validates a test configuration against a policy
func (pc *PolicyChecker) ValidatePolicy(policy *PolicyConfig, options lib.Options, scriptContent string) []PolicyViolation {
	var violations []PolicyViolation

	// Check required thresholds
	if policy.RequireThresholds {
		if options.Thresholds == nil || len(options.Thresholds) == 0 {
			violations = append(violations, PolicyViolation{
				Type:        "thresholds",
				Description: "Missing required thresholds",
			})
		}
	}

	// Check required tags
	for _, requiredTag := range policy.RequiredTags {
		found := false
		if options.RunTags != nil {
			for tag := range options.RunTags {
				if tag == requiredTag {
					found = true
					break
				}
			}
		}
		if !found {
			violations = append(violations, PolicyViolation{
				Type:        "tags",
				Description: fmt.Sprintf("Missing required tag: %s", requiredTag),
			})
		}
	}

	// Check disallowed strings (literal matching)
	for _, str := range policy.DisallowedStrings {
		if strings.Contains(scriptContent, str) {
			violations = append(violations, PolicyViolation{
				Type:        "string",
				Description: fmt.Sprintf("Found disallowed string: %s", str),
			})
		}
	}

	// Check disallowed regex patterns
	for _, pattern := range policy.DisallowedRegex {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			// Skip invalid regex patterns but log them
			violations = append(violations, PolicyViolation{
				Type:        "pattern",
				Description: fmt.Sprintf("Invalid regex pattern in policy: %s", pattern),
			})
			continue
		}

		if regex.MatchString(scriptContent) {
			violations = append(violations, PolicyViolation{
				Type:        "pattern",
				Description: fmt.Sprintf("Found disallowed pattern: %s", pattern),
			})
		}
	}

	return violations
}

// PrintPolicyResult prints the policy validation result to the provided writer
func PrintPolicyResult(result *PolicyResult, stdout io.Writer) error {
	if !result.Used {
		return nil // No policy was used, nothing to print
	}

	// Always show which policy file is being used
	if _, err := fmt.Fprintf(stdout, "✓ Using policy file: %s\n", result.PolicyFile); err != nil {
		return err
	}

	// If there are violations, print them
	if len(result.Violations) > 0 {
		if _, err := fmt.Fprintf(stdout, "⚠️ Policy Violations:\n"); err != nil {
			return err
		}
		for _, violation := range result.Violations {
			if _, err := fmt.Fprintf(stdout, "- %s\n", violation.Description); err != nil {
				return err
			}
		}
		return nil
	}

	return nil
}

// CheckPolicy performs complete policy checking for a script
func (pc *PolicyChecker) CheckPolicy(scriptPath, explicitPolicyPath string, options lib.Options, scriptContent string) (*PolicyResult, error) {
	result := &PolicyResult{Used: false}

	var policyPath string
	var policyExists bool

	// Determine which policy file to use
	if explicitPolicyPath != "" {
		// User provided explicit policy path
		policyPath = explicitPolicyPath
		exists, err := fsext.Exists(pc.fs, policyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to check policy file: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("❌ Policy file not found: %s", policyPath)
		}
		policyExists = true
	} else {
		// Look for k6policy.json in script directory
		policyPath, policyExists = pc.FindPolicyFile(scriptPath)
	}

	// If no policy file found and none explicitly provided, skip silently
	if !policyExists {
		return result, nil
	}

	// Load and validate policy
	policy, err := pc.LoadPolicy(policyPath)
	if err != nil {
		return nil, err
	}

	violations := pc.ValidatePolicy(policy, options, scriptContent)

	result.PolicyFile = policyPath
	result.Violations = violations
	result.Used = true

	return result, nil
}

// PrintPolicyRules prints the rules being evaluated from a policy
func PrintPolicyRules(policy *PolicyConfig, stdout io.Writer) error {
	rules := []string{}

	if policy.RequireThresholds {
		rules = append(rules, "Required thresholds")
	}

	if len(policy.RequiredTags) > 0 {
		rules = append(rules, fmt.Sprintf("Required tags: %s", strings.Join(policy.RequiredTags, ", ")))
	}

	if len(policy.DisallowedStrings) > 0 {
		rules = append(rules, fmt.Sprintf("Disallowed strings: %s", strings.Join(policy.DisallowedStrings, ", ")))
	}

	if len(policy.DisallowedRegex) > 0 {
		rules = append(rules, fmt.Sprintf("Disallowed regex patterns: %s", strings.Join(policy.DisallowedRegex, ", ")))
	}

	if _, err := fmt.Fprintf(stdout, "Policy Rules:\n"); err != nil {
		return err
	}

	if len(rules) > 0 {
		for _, rule := range rules {
			if _, err := fmt.Fprintf(stdout, "- %s\n", rule); err != nil {
				return err
			}
		}
	} else {
		if _, err := fmt.Fprintf(stdout, "- No policy rules defined\n"); err != nil {
			return err
		}
	}

	return nil
}
