package docker

import (
	"testing"

	"github.com/fabienpiette/docker-stats-exporter/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilter_NoRules(t *testing.T) {
	f, err := NewFilter(config.FiltersConfig{})
	require.NoError(t, err)

	c := &Container{Name: "anything", Image: "anything"}
	assert.True(t, f.Match(c), "no rules should match everything")
}

func TestFilter_IncludeByName(t *testing.T) {
	f, err := NewFilter(config.FiltersConfig{
		Include: config.FilterSet{Names: []string{"^web-.*"}},
	})
	require.NoError(t, err)

	assert.True(t, f.Match(&Container{Name: "web-frontend"}))
	assert.True(t, f.Match(&Container{Name: "web-api"}))
	assert.False(t, f.Match(&Container{Name: "db-postgres"}))
}

func TestFilter_ExcludeByName(t *testing.T) {
	f, err := NewFilter(config.FiltersConfig{
		Exclude: config.FilterSet{Names: []string{"^test-.*"}},
	})
	require.NoError(t, err)

	assert.True(t, f.Match(&Container{Name: "web-app"}))
	assert.False(t, f.Match(&Container{Name: "test-runner"}))
}

func TestFilter_ExcludeTakesPrecedence(t *testing.T) {
	f, err := NewFilter(config.FiltersConfig{
		Include: config.FilterSet{Names: []string{".*"}},
		Exclude: config.FilterSet{Names: []string{"^secret-.*"}},
	})
	require.NoError(t, err)

	assert.True(t, f.Match(&Container{Name: "web-app"}))
	assert.False(t, f.Match(&Container{Name: "secret-service"}))
}

func TestFilter_IncludeByImage(t *testing.T) {
	f, err := NewFilter(config.FiltersConfig{
		Include: config.FilterSet{Images: []string{"nginx:.*", "redis:.*"}},
	})
	require.NoError(t, err)

	assert.True(t, f.Match(&Container{Name: "a", Image: "nginx:latest"}))
	assert.True(t, f.Match(&Container{Name: "b", Image: "redis:7"}))
	assert.False(t, f.Match(&Container{Name: "c", Image: "postgres:15"}))
}

func TestFilter_IncludeByLabel(t *testing.T) {
	f, err := NewFilter(config.FiltersConfig{
		Include: config.FilterSet{Labels: []string{"monitoring=true"}},
	})
	require.NoError(t, err)

	assert.True(t, f.Match(&Container{
		Name:   "web",
		Labels: map[string]string{"monitoring": "true"},
	}))
	assert.False(t, f.Match(&Container{
		Name:   "db",
		Labels: map[string]string{"monitoring": "false"},
	}))
	assert.False(t, f.Match(&Container{
		Name:   "worker",
		Labels: map[string]string{},
	}))
}

func TestFilter_ExcludeByLabel(t *testing.T) {
	f, err := NewFilter(config.FiltersConfig{
		Exclude: config.FilterSet{Labels: []string{"internal=true"}},
	})
	require.NoError(t, err)

	assert.True(t, f.Match(&Container{Name: "web", Labels: map[string]string{}}))
	assert.False(t, f.Match(&Container{
		Name:   "internal-svc",
		Labels: map[string]string{"internal": "true"},
	}))
}

func TestFilter_LabelKeyOnly(t *testing.T) {
	f, err := NewFilter(config.FiltersConfig{
		Include: config.FilterSet{Labels: []string{"monitor"}},
	})
	require.NoError(t, err)

	// Key-only match: label exists with any value
	assert.True(t, f.Match(&Container{
		Name:   "web",
		Labels: map[string]string{"monitor": "yes"},
	}))
	assert.True(t, f.Match(&Container{
		Name:   "db",
		Labels: map[string]string{"monitor": ""},
	}))
	assert.False(t, f.Match(&Container{
		Name:   "worker",
		Labels: map[string]string{},
	}))
}

func TestFilter_InvalidRegex(t *testing.T) {
	_, err := NewFilter(config.FiltersConfig{
		Include: config.FilterSet{Names: []string{"[invalid"}},
	})
	assert.Error(t, err)
}
