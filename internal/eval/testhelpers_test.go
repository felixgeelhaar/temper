package eval

import (
	"os"
	"path/filepath"
)

func writeFileImpl(dir, name, content string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
}
