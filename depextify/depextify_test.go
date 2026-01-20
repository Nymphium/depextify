package depextify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDo(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]struct{}
	}{
		{
			name: "simple case",
			content: `
echo "hello"
ls -l
cat file.txt
`,
			expected: map[string]struct{}{
				"ls":  {},
				"cat": {},
			},
		},
		{
			name: "with function",
			content: `
my_func() {
  echo "hello"
}

ls -l
my_func
`,
			expected: map[string]struct{}{
				"ls": {},
			},
		},
		{
			name: "with builtins",
			content: `
if true; then
  cd /tmp
fi
`,
			expected: map[string]struct{}{},
		},
		{
			name:     "empty file",
			content:  "",
			expected: map[string]struct{}{},
		},
		{
			name: "complex case",
			content: `
#!/bin/sh

# this is a comment
echo "hello world"

my_func() {
  echo "in function"
  grep "pattern" file
}

if [ -f "file" ]; then
  cat file | wc -l
fi

my_func

curl -s https://example.com
`,
			expected: map[string]struct{}{
				"grep": {},
				"cat":  {},
				"wc":   {},
				"curl": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.sh")
			f, err := os.Create(filePath)
			require.NoError(t, err)
			_, err = f.WriteString(tt.content)
			require.NoError(t, err)
			f.Seek(0, 0)
			defer f.Close()

			actual, err := Do(f)
			require.NoError(t, err)

			require.Equal(t, tt.expected, actual)
		})
	}
}
