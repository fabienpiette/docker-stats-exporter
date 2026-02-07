package docker

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fabienpiette/docker-stats-exporter/pkg/config"
)

// Filter decides whether a container should be included in metrics collection.
// Exclude rules take precedence over include rules.
type Filter struct {
	includeLabels map[string]string
	excludeLabels map[string]string
	includeNames  []*regexp.Regexp
	excludeNames  []*regexp.Regexp
	includeImages []*regexp.Regexp
	excludeImages []*regexp.Regexp
	hasIncludes   bool
}

// NewFilter compiles filter patterns from configuration. Returns an error if
// any regex pattern is invalid.
func NewFilter(cfg config.FiltersConfig) (*Filter, error) {
	f := &Filter{}

	f.includeLabels = parseLabels(cfg.Include.Labels)
	f.excludeLabels = parseLabels(cfg.Exclude.Labels)

	var err error
	if f.includeNames, err = compilePatterns(cfg.Include.Names); err != nil {
		return nil, fmt.Errorf("compiling include name patterns: %w", err)
	}
	if f.excludeNames, err = compilePatterns(cfg.Exclude.Names); err != nil {
		return nil, fmt.Errorf("compiling exclude name patterns: %w", err)
	}
	if f.includeImages, err = compilePatterns(cfg.Include.Images); err != nil {
		return nil, fmt.Errorf("compiling include image patterns: %w", err)
	}
	if f.excludeImages, err = compilePatterns(cfg.Exclude.Images); err != nil {
		return nil, fmt.Errorf("compiling exclude image patterns: %w", err)
	}

	f.hasIncludes = len(f.includeLabels) > 0 || len(f.includeNames) > 0 || len(f.includeImages) > 0

	return f, nil
}

// Match returns true if the container should be collected.
// Exclude rules are checked first — if any exclude matches, the container is skipped.
// If include rules exist, at least one must match.
func (f *Filter) Match(c *Container) bool {
	// Check excludes first (they take precedence)
	if matchesLabels(c.Labels, f.excludeLabels) {
		return false
	}
	if matchesAny(c.Name, f.excludeNames) {
		return false
	}
	if matchesAny(c.Image, f.excludeImages) {
		return false
	}

	// If no include rules, everything passes
	if !f.hasIncludes {
		return true
	}

	// At least one include rule must match
	if matchesLabels(c.Labels, f.includeLabels) {
		return true
	}
	if matchesAny(c.Name, f.includeNames) {
		return true
	}
	if matchesAny(c.Image, f.includeImages) {
		return true
	}

	return false
}

func parseLabels(raw []string) map[string]string {
	labels := make(map[string]string, len(raw))
	for _, l := range raw {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		} else if len(parts) == 1 && parts[0] != "" {
			labels[parts[0]] = "" // key-only match
		}
	}
	return labels
}

func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

func matchesLabels(containerLabels map[string]string, filterLabels map[string]string) bool {
	for key, val := range filterLabels {
		cv, ok := containerLabels[key]
		if !ok {
			continue
		}
		if val == "" || cv == val {
			return true
		}
	}
	return false
}

func matchesAny(s string, patterns []*regexp.Regexp) bool {
	for _, re := range patterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
