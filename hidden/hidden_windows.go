//go:build windows

package hidden

import "path/filepath"
import "syscall"

const dotCharacter = 46

func IsHidden(path string) (bool, error) {
	if path[0] == dotCharacter {
		return true, nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}

	pointer, err := syscall.UTF16PtrFromString(`\\?\` + absPath)
	if err != nil {
		return false, err
	}

	attributes, err := syscall.GetFileAttributes(pointer)
	if err != nil {
		return false, err
	}

	return attributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0, nil
}
