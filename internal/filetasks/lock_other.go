//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package filetasks

import "os"

func lockFile(string) (*os.File, error) { return nil, ErrStore }
