//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package distribution

import (
	"errors"
	"os"
)

func lockFile(path string) (*os.File, error) {
	return nil, errors.New("persistent mode requires a supported Unix file lock")
}
