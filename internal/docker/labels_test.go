package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractLabels(t *testing.T) {
	c := &Container{
		Name:  "my-web-app",
		Image: "nginx:1.25",
		Labels: map[string]string{
			"com.docker.compose.service": "web",
			"com.docker.compose.project": "myproject",
		},
	}

	labels := ExtractLabels(c)
	assert.Equal(t, "my-web-app", labels.ContainerName)
	assert.Equal(t, "web", labels.ComposeService)
	assert.Equal(t, "myproject", labels.ComposeProject)
	assert.Equal(t, "nginx:1.25", labels.Image)
}

func TestExtractLabels_NoComposeLabels(t *testing.T) {
	c := &Container{
		Name:   "standalone",
		Image:  "redis:7",
		Labels: map[string]string{},
	}

	labels := ExtractLabels(c)
	assert.Equal(t, "standalone", labels.ContainerName)
	assert.Equal(t, "", labels.ComposeService)
	assert.Equal(t, "", labels.ComposeProject)
	assert.Equal(t, "redis:7", labels.Image)
}

func TestLabelValues(t *testing.T) {
	labels := ContainerLabels{
		ContainerName:  "web",
		ComposeService: "svc",
		ComposeProject: "proj",
		Image:          "img",
	}

	values := labels.Values()
	assert.Equal(t, []string{"web", "svc", "proj", "img"}, values)
}

func TestSanitizeLabelValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with spaces", "with_spaces"},
		{"with\ttab", "with_tab"},
		{"with/slash", "with/slash"},
		{"with:colon", "with:colon"},
		{"  trimmed  ", "trimmed"},
		{"special!chars#here", "special_chars_here"},
		{"nginx:1.25-alpine", "nginx:1.25-alpine"},
		{"user@domain", "user@domain"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeLabelValue(tt.input))
		})
	}
}

func TestLabelNames(t *testing.T) {
	names := LabelNames()
	assert.Equal(t, []string{"container_name", "compose_service", "compose_project", "image"}, names)
}
