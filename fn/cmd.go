package fn

import "strings"

type VarCheckCMDOut interface {
	*error | string
}

func MatchCMDOut(cmdout []byte, substr string) bool {
	return strings.Contains(string(cmdout), substr)
}

// func CheckCMDOut(cmdout []byte, err error, args ...string) (string, bool, error) {
// 	// var matched string
// 	// if len(args) > 0 {
// 	// 	matched = args[0]
// 	// }
// 	for _, arg := range args {
// 		if strings.Contains(string(cmdout), arg) {
// 			return string(cmdout), true, nil
// 		}
// 	}
// 	// if len(matched) > 0 {
// 	// 	if strings.Contains(string(cmdout), matched) {
// 	// 		return string(cmdout), nil
// 	// 	}
// 	// }
// 	return string(cmdout), false, err
// }
