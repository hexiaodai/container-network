package fn

import (
	"fmt"
	"os"
	"strings"
)

func Args(name string) (out string) {
	args := os.Args[1:]
	for _, arg := range args {
		if strings.HasPrefix(arg, fmt.Sprintf("--%v=", name)) {
			res := strings.Split(arg, "=")
			if len(res) > 2 {
				panic("failed to parse args. arg: " + arg)
			}
			out = res[1]
			return
		}
	}
	return
}
