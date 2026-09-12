//go:build windows

package tty

import (
	"bufio"
	"os"
	"strings"
)

func Prompt(question string) (bool, error) {
	in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer in.Close()
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return false, err
	}
	defer out.Close()
	if _, err := out.WriteString(question); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
