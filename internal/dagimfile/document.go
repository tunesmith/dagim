// SPDX-License-Identifier: GPL-3.0-or-later

package dagimfile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/tunesmith/dagim/internal/diagnostic"
)

// IsCollection reserves a distinct header, independent of the filename. Older
// graphs without headers continue to be accepted by the graph parser.
func IsCollection(input string) bool {
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line == "# dagim collection" || strings.HasPrefix(line, "# dagim collection ")
		}
	}
	return false
}

func PathIsCollection(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return IsCollection(string(data)), nil
}

// SaveTextIfUnchanged uses the same atomic replacement as graph saves and
// refuses to overwrite a document changed since it was read.
func SaveTextIfUnchanged(path, text string, expected []byte) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, expected) {
		return diagnostic.NewError("file_changed", "file changed on disk: "+path, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := writeAtomicTemp(path, text, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func CreateTextAtomic(path, text string) error {
	tmp, err := writeAtomicTemp(path, text, 0o644)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := os.Link(tmp, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
