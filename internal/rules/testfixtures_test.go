package rules_test

import (
	"strings"
	"testing"

	"mflint/internal/parser"
	"mflint/internal/rules"
)

// parseYAML is a test helper: parses a single YAML doc (or multiple,
// separated by ---) into resources, failing the test on error.
func parseYAML(t *testing.T, yaml string) []parser.Resource {
	t.Helper()
	res, err := parser.Parse(strings.NewReader(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("parsing test fixture: %v", err)
	}
	return res
}

// countViolations runs every registered rule against resources and counts
// how many violations rule ruleID produced.
func countViolations(t *testing.T, ruleID string, resources []parser.Resource) int {
	t.Helper()
	count := 0
	for _, v := range rules.RunAll(resources) {
		if v.RuleID == ruleID {
			count++
		}
	}
	return count
}
