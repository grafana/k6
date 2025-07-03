package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/metrics"
)

func TestPolicyChecker_LoadPolicy(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	// Create a valid policy file
	policyContent := `{
		"requireThresholds": true,
		"requiredTags": ["team", "env"],
		"disallowedPatterns": ["console.log", "sleep\\(\\d{5,}\\)"]
	}`
	require.NoError(t, fsext.WriteFile(fs, "policy.json", []byte(policyContent), 0o644))

	policy, err := pc.LoadPolicy("policy.json")
	require.NoError(t, err)

	assert.True(t, policy.RequireThresholds)
	assert.Equal(t, []string{"team", "env"}, policy.RequiredTags)
	assert.Equal(t, []string{"console.log", "sleep\\(\\d{5,}\\)"}, policy.DisallowedPatterns)
}

func TestPolicyChecker_LoadPolicy_InvalidJSON(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	// Create an invalid JSON file
	invalidJSON := `{
		"requireThresholds": true,
		"requiredTags": ["team", "env"
	}` // Missing closing bracket
	require.NoError(t, fsext.WriteFile(fs, "invalid.json", []byte(invalidJSON), 0o644))

	_, err := pc.LoadPolicy("invalid.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse policy file")
}

func TestPolicyChecker_FindPolicyFile(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	// Create a k6policy.json file
	policyContent := `{"requireThresholds": true}`
	require.NoError(t, fsext.WriteFile(fs, "test/k6policy.json", []byte(policyContent), 0o644))

	// Test finding policy file
	policyPath, found := pc.FindPolicyFile("test/script.js")
	assert.True(t, found)
	assert.Equal(t, "test/k6policy.json", policyPath)

	// Test when policy file doesn't exist
	policyPath, found = pc.FindPolicyFile("other/script.js")
	assert.False(t, found)
	assert.Empty(t, policyPath)
}

func TestPolicyChecker_ValidatePolicy_RequireThresholds(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	policy := &PolicyConfig{
		RequireThresholds: true,
	}

	// Test with no thresholds
	options := lib.Options{}
	violations := pc.ValidatePolicy(policy, options, "")
	assert.Len(t, violations, 1)
	assert.Equal(t, "Missing required thresholds", violations[0].Description)

	// Test with thresholds
	options.Thresholds = map[string]metrics.Thresholds{
		"http_req_duration": {},
	}
	violations = pc.ValidatePolicy(policy, options, "")
	assert.Len(t, violations, 0)
}

func TestPolicyChecker_ValidatePolicy_RequiredTags(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	policy := &PolicyConfig{
		RequiredTags: []string{"team", "env"},
	}

	// Test with no tags
	options := lib.Options{}
	violations := pc.ValidatePolicy(policy, options, "")
	assert.Len(t, violations, 2)
	assert.Contains(t, violations[0].Description, "Missing required tag: team")
	assert.Contains(t, violations[1].Description, "Missing required tag: env")

	// Test with some tags
	tagMap := map[string]string{"team": "backend"}
	options.RunTags = tagMap
	violations = pc.ValidatePolicy(policy, options, "")
	assert.Len(t, violations, 1)
	assert.Contains(t, violations[0].Description, "Missing required tag: env")

	// Test with all required tags
	tagMap = map[string]string{"team": "backend", "env": "prod"}
	options.RunTags = tagMap
	violations = pc.ValidatePolicy(policy, options, "")
	assert.Len(t, violations, 0)
}

func TestPolicyChecker_ValidatePolicy_DisallowedPatterns(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	policy := &PolicyConfig{
		DisallowedPatterns: []string{"console.log", "sleep\\(\\d{5,}\\)"},
	}

	// Test with no violations
	scriptContent := `
		import http from 'k6/http';
		export default function() {
			http.get('https://example.com');
			sleep(1);
		}
	`
	violations := pc.ValidatePolicy(policy, lib.Options{}, scriptContent)
	assert.Len(t, violations, 0)

	// Test with console.log violation
	scriptContentWithConsole := `
		import http from 'k6/http';
		export default function() {
			console.log('Debug message');
			http.get('https://example.com');
		}
	`
	violations = pc.ValidatePolicy(policy, lib.Options{}, scriptContentWithConsole)
	assert.Len(t, violations, 1)
	assert.Contains(t, violations[0].Description, "Found disallowed pattern: console.log")

	// Test with long sleep violation
	scriptContentWithLongSleep := `
		import { sleep } from 'k6';
		export default function() {
			sleep(60000);
		}
	`
	violations = pc.ValidatePolicy(policy, lib.Options{}, scriptContentWithLongSleep)
	assert.Len(t, violations, 1)
	assert.Contains(t, violations[0].Description, "Found disallowed pattern: sleep\\(\\d{5,}\\)")

	// Test with multiple violations
	scriptContentWithBoth := `
		import { sleep } from 'k6';
		export default function() {
			console.log('Debug');
			sleep(60000);
		}
	`
	violations = pc.ValidatePolicy(policy, lib.Options{}, scriptContentWithBoth)
	assert.Len(t, violations, 2)
}

func TestPolicyChecker_ValidatePolicy_InvalidRegex(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	policy := &PolicyConfig{
		DisallowedPatterns: []string{"[invalid"},
	}

	violations := pc.ValidatePolicy(policy, lib.Options{}, "test content")
	assert.Len(t, violations, 1)
	assert.Contains(t, violations[0].Description, "Invalid regex pattern in policy")
}

func TestPolicyChecker_CheckPolicy_NoPolicy(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	// Create a script without policy
	require.NoError(t, fsext.WriteFile(fs, "script.js", []byte("test content"), 0o644))

	result, err := pc.CheckPolicy("script.js", "", lib.Options{}, "test content")
	require.NoError(t, err)
	assert.False(t, result.Used)
	assert.Empty(t, result.Violations)
}

func TestPolicyChecker_CheckPolicy_AutoDetect(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	// Create script and policy in same directory
	require.NoError(t, fsext.WriteFile(fs, "test/script.js", []byte("console.log('test')"), 0o644))
	policyContent := `{
		"requireThresholds": false,
		"requiredTags": [],
		"disallowedPatterns": ["console.log"]
	}`
	require.NoError(t, fsext.WriteFile(fs, "test/k6policy.json", []byte(policyContent), 0o644))

	result, err := pc.CheckPolicy("test/script.js", "", lib.Options{}, "console.log('test')")
	require.NoError(t, err)
	assert.True(t, result.Used)
	assert.Equal(t, "test/k6policy.json", result.PolicyFile)
	assert.Len(t, result.Violations, 1)
	assert.Contains(t, result.Violations[0].Description, "Found disallowed pattern: console.log")
}

func TestPolicyChecker_CheckPolicy_ExplicitPolicy(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	// Create script and explicit policy file
	require.NoError(t, fsext.WriteFile(fs, "script.js", []byte("test content"), 0o644))
	policyContent := `{
		"requireThresholds": true,
		"requiredTags": ["team"],
		"disallowedPatterns": []
	}`
	require.NoError(t, fsext.WriteFile(fs, "custom-policy.json", []byte(policyContent), 0o644))

	result, err := pc.CheckPolicy("script.js", "custom-policy.json", lib.Options{}, "test content")
	require.NoError(t, err)
	assert.True(t, result.Used)
	assert.Equal(t, "custom-policy.json", result.PolicyFile)
	assert.Len(t, result.Violations, 2) // Missing thresholds and missing team tag
}

func TestPolicyChecker_CheckPolicy_ExplicitPolicyNotFound(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	pc := NewPolicyChecker(fs)

	// Create script but no policy file
	require.NoError(t, fsext.WriteFile(fs, "script.js", []byte("test content"), 0o644))

	_, err := pc.CheckPolicy("script.js", "nonexistent-policy.json", lib.Options{}, "test content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "❌ Policy file not found: nonexistent-policy.json")
}

func TestPrintPolicyResult(t *testing.T) {
	t.Parallel()

	// Test no policy used
	result := &PolicyResult{Used: false}
	var output strings.Builder
	err := PrintPolicyResult(result, &output)
	require.NoError(t, err)
	assert.Empty(t, output.String())

	// Test policy used with no violations
	result = &PolicyResult{
		Used:       true,
		PolicyFile: "k6policy.json",
		Violations: []PolicyViolation{},
	}
	output.Reset()
	err = PrintPolicyResult(result, &output)
	require.NoError(t, err)
	assert.Contains(t, output.String(), "✓ Using policy file: k6policy.json")

	// Test policy used with violations
	result = &PolicyResult{
		Used:       true,
		PolicyFile: "k6policy.json",
		Violations: []PolicyViolation{
			{Type: "tags", Description: "Missing required tag: team"},
			{Type: "pattern", Description: "Found disallowed pattern: console.log"},
		},
	}
	output.Reset()
	err = PrintPolicyResult(result, &output)
	require.NoError(t, err)
	outputStr := output.String()
	assert.Contains(t, outputStr, "✓ Using policy file: k6policy.json")
	assert.Contains(t, outputStr, "⚠️ Policy Violations:")
	assert.Contains(t, outputStr, "- Missing required tag: team")
	assert.Contains(t, outputStr, "- Found disallowed pattern: console.log")
}

func TestPrintPolicyRules(t *testing.T) {
	t.Parallel()

	policy := &PolicyConfig{
		RequireThresholds:  true,
		RequiredTags:       []string{"team", "env"},
		DisallowedPatterns: []string{"console.log", "debugger"},
	}

	var output strings.Builder
	err := PrintPolicyRules(policy, &output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Policy rules:")
	assert.Contains(t, outputStr, "- Required thresholds")
	assert.Contains(t, outputStr, "- Required tags: team, env")
	assert.Contains(t, outputStr, "- Disallowed patterns: console.log, debugger")

	// Test empty policy
	emptyPolicy := &PolicyConfig{}
	output.Reset()
	err = PrintPolicyRules(emptyPolicy, &output)
	require.NoError(t, err)
	assert.Empty(t, output.String())
}
