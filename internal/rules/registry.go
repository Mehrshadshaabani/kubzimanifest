// Package rules implements mflint's Kubernetes manifest lint rules. Each
// rule lives in its own file and self-registers via init(), so adding a new
// rule never requires touching this file or any other rule.
package rules

import (
	"sort"
	"sync"

	"mflint/internal/parser"
)

// Severity classifies how serious a violation is.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// severityRank orders severities from most to least serious, for sorting.
var severityRank = map[Severity]int{
	SeverityCritical: 0,
	SeverityWarning:  1,
	SeverityInfo:     2,
}

// Violation is one rule failing against one resource.
type Violation struct {
	RuleID     string             `json:"ruleId"`
	Title      string             `json:"title"`
	Severity   Severity           `json:"severity"`
	Resource   parser.ResourceRef `json:"resource"`
	Message    string             `json:"message"`
	Suggestion string             `json:"suggestion"`
	DocLink    string             `json:"docLink"`
}

// Rule is one independent, pluggable lint check. all contains every resource
// parsed in the current run, for rules that need cross-resource context
// (e.g. "is there a NetworkPolicy covering this namespace").
type Rule interface {
	ID() string
	Title() string
	Severity() Severity
	DocLink() string
	Check(target parser.Resource, all []parser.Resource) []Violation
}

type funcRule struct {
	id       string
	title    string
	severity Severity
	docLink  string
	check    func(target parser.Resource, all []parser.Resource) []Violation
}

func (f funcRule) ID() string         { return f.id }
func (f funcRule) Title() string      { return f.title }
func (f funcRule) Severity() Severity { return f.severity }
func (f funcRule) DocLink() string    { return f.docLink }
func (f funcRule) Check(target parser.Resource, all []parser.Resource) []Violation {
	return f.check(target, all)
}

var (
	mu       sync.Mutex
	registry = map[string]Rule{}
)

// newRule builds a Rule from a plain check function plus its metadata.
func newRule(id, title string, severity Severity, docLink string, check func(target parser.Resource, all []parser.Resource) []Violation) Rule {
	return funcRule{id: id, title: title, severity: severity, docLink: docLink, check: check}
}

// Register adds a rule to the global registry. Rule files call this from
// their own init(). Panics on a duplicate ID, since that's a programming
// error caught at process startup / test time, not runtime input.
func Register(r Rule) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[r.ID()]; exists {
		panic("rules: duplicate rule id " + r.ID())
	}
	registry[r.ID()] = r
}

// All returns every registered rule, sorted by ID for deterministic output.
func All() []Rule {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Rule, 0, len(registry))
	for _, r := range registry {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// RunAll checks every resource in resources against every registered rule
// and returns all violations, sorted by severity then rule ID then resource.
func RunAll(resources []parser.Resource) []Violation {
	rules := All()
	var violations []Violation
	for _, res := range resources {
		for _, rule := range rules {
			violations = append(violations, rule.Check(res, resources)...)
		}
	}
	sort.SliceStable(violations, func(i, j int) bool {
		if severityRank[violations[i].Severity] != severityRank[violations[j].Severity] {
			return severityRank[violations[i].Severity] < severityRank[violations[j].Severity]
		}
		if violations[i].RuleID != violations[j].RuleID {
			return violations[i].RuleID < violations[j].RuleID
		}
		return violations[i].Resource.Name < violations[j].Resource.Name
	})
	return violations
}
