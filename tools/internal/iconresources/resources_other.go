//go:build !windows

package iconresources

import "fmt"

func loadGroups(string) ([]resourceGroup, error) {
	return nil, fmt.Errorf("Native Windows resource verification requires Windows")
}
