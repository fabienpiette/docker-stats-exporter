package docker

import (
	"testing"
)

func BenchmarkParseDockerStats(b *testing.B) {
	statsJSON := loadTestStatsJSON(b)
	containerJSON := testContainerJSON()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseDockerStats(statsJSON, containerJSON)
	}
}

func BenchmarkExtractLabels(b *testing.B) {
	c := &Container{
		Name:  "my-web-app",
		Image: "nginx:1.25-alpine",
		Labels: map[string]string{
			"com.docker.compose.service": "web",
			"com.docker.compose.project": "myproject",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractLabels(c)
	}
}

func BenchmarkSanitizeLabelValue(b *testing.B) {
	inputs := []string{
		"simple",
		"with spaces and special!chars",
		"nginx:1.25-alpine",
		"very-long-container-name-with-many-segments-and-parts",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SanitizeLabelValue(inputs[i%len(inputs)])
	}
}
