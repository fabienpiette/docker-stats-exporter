package docker

import (
	"regexp"
	"strings"
)

// Standard Docker Compose label keys.
const (
	LabelComposeService = "com.docker.compose.service"
	LabelComposeProject = "com.docker.compose.project"
)

// ContainerLabels holds the standard label set emitted with every metric.
type ContainerLabels struct {
	ContainerName  string
	ComposeService string
	ComposeProject string
	Image          string
}

// LabelNames returns the label keys in a fixed order.
func LabelNames() []string {
	return []string{"container_name", "compose_service", "compose_project", "image"}
}

// ExtractLabels builds the standard label set from a Container.
func ExtractLabels(c *Container) ContainerLabels {
	return ContainerLabels{
		ContainerName:  SanitizeLabelValue(c.Name),
		ComposeService: SanitizeLabelValue(c.Labels[LabelComposeService]),
		ComposeProject: SanitizeLabelValue(c.Labels[LabelComposeProject]),
		Image:          SanitizeLabelValue(c.Image),
	}
}

// ExtractLabelsFromStats builds the standard label set from a Stats.
func ExtractLabelsFromStats(s *Stats) ContainerLabels {
	return ContainerLabels{
		ContainerName:  SanitizeLabelValue(s.Name),
		ComposeService: SanitizeLabelValue(s.Labels[LabelComposeService]),
		ComposeProject: SanitizeLabelValue(s.Labels[LabelComposeProject]),
		Image:          SanitizeLabelValue(s.Image),
	}
}

// Values returns label values in the same order as LabelNames.
func (l ContainerLabels) Values() []string {
	return []string{l.ContainerName, l.ComposeService, l.ComposeProject, l.Image}
}

var invalidLabelChars = regexp.MustCompile(`[^a-zA-Z0-9_:/.@=-]`)

// SanitizeLabelValue replaces characters that aren't safe in Prometheus label values.
func SanitizeLabelValue(s string) string {
	s = strings.TrimSpace(s)
	return invalidLabelChars.ReplaceAllString(s, "_")
}
