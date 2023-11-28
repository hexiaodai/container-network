package fn

import (
	"strings"
)

func CheckCMDOut(cmdout []byte, err error, args ...string) (string, error) {
	var matched string
	if len(args) > 0 {
		matched = args[0]
	}
	if len(matched) > 0 {
		if strings.Contains(string(cmdout), matched) {
			return string(cmdout), nil
		}
	}
	return string(cmdout), err
}
