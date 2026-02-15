package link

import (
	"bufio"
	"errors"
	"os"
	"sort"
	"strings"
)

// loadManifest reads the manifest file and returns deduplicated, sorted paths.
// Returns an empty slice (no error) if the file does not exist.
func loadManifest(path string) (paths []string, err error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	seen := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !seen[line] {
			seen[line] = true
			paths = append(paths, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// saveManifest writes the given paths to the manifest file, one per line.
func saveManifest(path string, targets []string) error {
	content := strings.Join(targets, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}
