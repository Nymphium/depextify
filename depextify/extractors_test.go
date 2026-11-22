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

func TestExtractors_Variables(t *testing.T) {
	tests := []struct {
		name      string
		extractor Extractor
		content   string
		expected  map[string][]PosInfo
	}{
		{
			"Makefile variable",
			&MakefileExtractor{},
			"CC = gcc\nLD = ld\nall:\n\t$(CC) main.c\n\t${MAKE} -C sub",
			map[string][]PosInfo{
				"gcc":   {{line: 1, col: 6, len: 3}},
				"$MAKE": {{line: 5, col: 2, len: 7}},
			},
		},
		{
			"Dockerfile ARG and ENV",
			&DockerfileExtractor{},
			"ARG CC=gcc\nENV GIT git\nENV UNUSED tool\nRUN $CC main.c && ${GIT} clone x",
			map[string][]PosInfo{
				"gcc": {{line: 1, col: 8, len: 3}},
				"git": {{line: 2, col: 9, len: 3}},
			},
		},
		{
			"YAML env",
			&YAMLExtractor{},
			"env:\n  CC: gcc\n  UNUSED: tool\nsteps:\n  - run: $CC build",
			map[string][]PosInfo{
				"gcc": {{line: 2, col: 7, len: 3}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.extractor.Extract([]byte(tt.content))
			require.NoError(t, err)
			require.Equal(t, tt.expected, res)
		})
	}
}

func TestYAMLExtractorCountsCmdMappingOnce(t *testing.T) {
	res, err := (&YAMLExtractor{}).Extract([]byte("tasks:\n  build:\n    cmds:\n      - cmd: mytool x\n"))
	require.NoError(t, err)
	require.Equal(t, []PosInfo{{line: 4, col: 14, len: 6}}, res["mytool"])
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
