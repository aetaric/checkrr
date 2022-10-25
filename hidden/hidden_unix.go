//go:build !windows

package hidden

import "path/filepath"

const dotCharacter = 46

func IsHidden(path string) (bool, error) {
	return filepath.Base(path)[0] == dotCharacter, nil
}
