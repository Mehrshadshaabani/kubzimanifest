package rules

import (
	"fmt"
	"strings"

	"mflint/internal/parser"
)

// imageTag returns the tag portion of an image reference and whether it is
// pinned by digest instead. An empty tag means the registry default
// ("latest") would be used.
func imageTag(image string) (tag string, digestPinned bool) {
	if strings.Contains(image, "@sha256:") {
		return "", true
	}
	last := image
	if i := strings.LastIndex(image, "/"); i >= 0 {
		last = image[i+1:]
	}
	if i := strings.LastIndex(last, ":"); i >= 0 {
		return last[i+1:], false
	}
	return "", false
}

func init() {
	const (
		id       = "K8S-008"
		title    = "Image tag not pinned"
		severity = SeverityWarning
		docLink  = "https://mflint.dev/rules/K8S-008"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		wl, ok := parser.AsWorkload(target.Object)
		if !ok {
			return nil
		}
		var out []Violation
		for _, c := range parser.AllContainers(wl.PodTemplate.Spec) {
			tag, digestPinned := imageTag(c.Image)
			if digestPinned || (tag != "" && tag != "latest") {
				continue
			}
			reason := "no tag specified (defaults to latest)"
			if tag == "latest" {
				reason = "tag is explicitly 'latest'"
			}
			out = append(out, Violation{
				RuleID:     id,
				Title:      title,
				Severity:   severity,
				DocLink:    docLink,
				Resource:   target.Ref,
				Message:    fmt.Sprintf("container %q image %q: %s", c.Name, c.Image, reason),
				Suggestion: "Pin the image to a specific version tag or, better, a digest (image@sha256:...) for reproducible deploys.",
			})
		}
		return out
	}))
}
