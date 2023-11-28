package f

import (
	"strings"
)

func CheckCMDOut(cmdout []byte, err error) (string, error) {
	if strings.Contains(string(cmdout), "already exists") {
		return string(cmdout), nil
	}
	return string(cmdout), err
}
