package platform_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/statix/statix/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFileSuccess(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")
	content := "hello world\nline two"
	err := os.WriteFile(filePath, []byte(content), 0600)
	require.NoError(t, err)

	var readContent string
	err = platform.ReadFile(filePath, func(r io.Reader) error {
		data, readErr := io.ReadAll(r)
		if readErr != nil {
			return readErr
		}
		readContent = string(data)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, content, readContent)
}

func TestReadFileNonExistent(t *testing.T) {
	err := platform.ReadFile("/path/does/not/exist/ever.txt", func(r io.Reader) error {
		return nil
	})
	require.Error(t, err)
}

func TestScanLines(t *testing.T) {
	input := "line1\nline2\nline3"
	r := strings.NewReader(input)

	var lines []string
	err := platform.ScanLines(r, func(line string) error {
		lines = append(lines, line)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"line1", "line2", "line3"}, lines)
}

func TestScanLinesCallbackError(t *testing.T) {
	input := "line1\nline2\nline3"
	r := strings.NewReader(input)

	targetErr := errors.New("stop iteration")
	err := platform.ScanLines(r, func(line string) error {
		if line == "line2" {
			return targetErr
		}
		return nil
	})

	require.ErrorIs(t, err, targetErr)
}
