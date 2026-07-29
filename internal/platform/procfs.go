package platform

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

// ReadFile opens path and passes an io.Reader to fn.
func ReadFile(path string, fn func(r io.Reader) error) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("platform: failed to open file %s: %w", path, err)
	}
	defer f.Close()

	if err := fn(f); err != nil {
		return fmt.Errorf("platform: error reading file %s: %w", path, err)
	}
	return nil
}

// ScanLines reads all lines of r, calling fn for each line.
func ScanLines(r io.Reader, fn func(line string) error) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if err := fn(scanner.Text()); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("platform: scanner error: %w", err)
	}
	return nil
}
