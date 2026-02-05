package depextify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractMakefile(t *testing.T) {
	content := `
all:
	echo "hello"
	ls -l

build:
	@go build
	-rm old_binary
`
	extractor := &MakefileExtractor{}
	res, err := extractor.Extract([]byte(content))
	require.NoError(t, err)
	
	require.Contains(t, res, "echo")
	require.Contains(t, res, "ls")
	require.Contains(t, res, "go")
	require.Contains(t, res, "rm")
}

func TestExtractDockerfile(t *testing.T) {
	content := `
FROM alpine
RUN apk add git
RUN go build \
    && ls -l
RUN ["echo", "hello"]
`
	extractor := &DockerfileExtractor{}
	res, err := extractor.Extract([]byte(content))
	require.NoError(t, err)

	require.Contains(t, res, "apk")
	require.Contains(t, res, "go")
	require.Contains(t, res, "ls")
	// "echo" in exec form is currently skipped by ExtractDockerfile implementation
	require.NotContains(t, res, "echo")
}

func TestExtractYAML(t *testing.T) {
	extractor := &YAMLExtractor{}
	t.Run("GitHub Actions", func(t *testing.T) {
		content := `
jobs:
  test:
    steps:
      - run: go test ./...
      - name: Build
        run: |
          go build
          ls -l
`
		res, err := extractor.Extract([]byte(content))
		require.NoError(t, err)
		require.Contains(t, res, "go")
		require.Contains(t, res, "ls")
	})

	t.Run("Taskfile", func(t *testing.T) {
		content := `
version: '3'
tasks:
  build:
    cmds:
      - go build
      - cmd: ls -l
`
		res, err := extractor.Extract([]byte(content))
		require.NoError(t, err)
		require.Contains(t, res, "go")
		require.Contains(t, res, "ls")
	})
}

func TestGetExtractor(t *testing.T) {
	tests := []struct {
		path     string
		expected Extractor
	}{
		{"Makefile", &MakefileExtractor{}},
		{"makefile", &MakefileExtractor{}},
		{"GNUmakefile", &MakefileExtractor{}},
		{"Dockerfile", &DockerfileExtractor{}},
		{"Dockerfile.dev", &DockerfileExtractor{}},
		{".github/workflows/ci.yml", &YAMLExtractor{}},
		{".github/workflows/deploy.yaml", &YAMLExtractor{}},
		{"script.sh", nil},
		{"Taskfile.yml", &YAMLExtractor{}},
	}

	for _, tt := range tests {
		got := GetExtractor(tt.path)
		if tt.expected == nil {
			require.Nil(t, got)
		} else {
			require.IsType(t, tt.expected, got)
		}
	}
}