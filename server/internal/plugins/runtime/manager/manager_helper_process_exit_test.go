package manager

import (
	"bufio"
	"os"
)

func helperDrainAndExit(scanner *bufio.Scanner, code int) {
	for scanner.Scan() {
	}
	os.Exit(code)
}
