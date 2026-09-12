//go:build !windows

package tty

import (
	"bufio"
	"os"
	"strings"
)

func Prompt(question string) (bool, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.WriteString(question); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
