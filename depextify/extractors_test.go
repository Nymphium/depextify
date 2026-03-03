package depextify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractors(t *testing.T) {
	t.Run("Makefile", func(t *testing.T) {
		extractor := &MakefileExtractor{}
		tests := []struct {
			name     string
			content  string
			expected []string
		}{
			{
				"simple",
				"all:\n\techo hello\n\tls -l",
				[]string{"echo", "ls"},
			},
			{
				"assignment",
				"CC = gcc\nall:\n\t$(CC) main.c",
				[]string{"gcc", "$CC", "CC"}, // CC is identified as a command in recipe
			},
			{
				"multiline",
				"all:\n\tcmd1 \\\n\t&& cmd2",
				[]string{"cmd1", "cmd2"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				res, err := extractor.Extract([]byte(tt.content))
				require.NoError(t, err)
				for _, cmd := range tt.expected {
					require.Contains(t, res, cmd)
				}
			})
		}
	})

	t.Run("Dockerfile", func(t *testing.T) {
		extractor := &DockerfileExtractor{}
		tests := []struct {
			name     string
			content  string
			expected []string
		}{
			{
				"RUN command",
				"FROM alpine\nRUN apk add git\nRUN go build",
				[]string{"apk", "go"},
			},
			{
				"ENV and ARG",
				"ARG CC=gcc\nENV GIT git\nRUN $(CC) clone",
				[]string{"gcc", "git", "$CC", "$GIT"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				res, err := extractor.Extract([]byte(tt.content))
				require.NoError(t, err)
				for _, cmd := range tt.expected {
					require.Contains(t, res, cmd)
				}
			})
		}
	})

	t.Run("YAML", func(t *testing.T) {
		extractor := &YAMLExtractor{}
		tests := []struct {
			name     string
			content  string
			expected []string
		}{
			{
				"GitHub Actions",
				"jobs:\n  test:\n    steps:\n      - run: go test",
				[]string{"go"},
			},
			{
				"Taskfile",
				"tasks:\n  build:\n    cmds:\n      - go build\n      - cmd: ls",
				[]string{"go", "ls"},
			},
			{
				"env block",
				"env:\n  CC: gcc\nsteps:\n  - run: $(CC) build",
				[]string{"gcc", "$CC"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				res, err := extractor.Extract([]byte(tt.content))
				require.NoError(t, err)
				for _, cmd := range tt.expected {
					require.Contains(t, res, cmd)
				}
			})
		}
	})
}

func TestGetExtractor(t *testing.T) {
	tests := []struct {
		path     string
		expected Extractor
	}{
		{"Makefile", &MakefileExtractor{}},
		{"Dockerfile", &DockerfileExtractor{}},
		{".github/workflows/ci.yml", &YAMLExtractor{}},
		{"Taskfile.yml", &YAMLExtractor{}},
		{"script.sh", nil},
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
