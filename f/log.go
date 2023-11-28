package f

import "fmt"

func Errorf(format string, a ...any) {
	fmt.Printf("\033[31m[error]\033[0m "+format+"\n", a...)
}
