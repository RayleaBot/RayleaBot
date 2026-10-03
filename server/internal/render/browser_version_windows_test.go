//go:build windows

package render

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func testBrowserVersion(_ context.Context, browserPath string) (string, error) {
	// Windows browsers may treat --version as a normal launch. Read PE metadata
	// in-process so diagnostics need neither a browser profile nor a shell.
	size, err := windows.GetFileVersionInfoSize(browserPath, nil)
	if err != nil {
		return "", err
	}
	if size == 0 {
		return "", fmt.Errorf("browser has no version resource")
	}
	data := make([]byte, size)
	if err := windows.GetFileVersionInfo(browserPath, 0, size, unsafe.Pointer(&data[0])); err != nil {
		return "", err
	}
	var info *windows.VS_FIXEDFILEINFO
	var length uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&data[0]), `\`, unsafe.Pointer(&info), &length); err != nil {
		return "", err
	}
	if info == nil || length < uint32(unsafe.Sizeof(*info)) || info.Signature != 0xFEEF04BD {
		return "", fmt.Errorf("browser has invalid fixed version information")
	}
	return fmt.Sprintf("FileVersion=%d.%d.%d.%d ProductVersion=%d.%d.%d.%d",
		info.FileVersionMS>>16, info.FileVersionMS&0xffff,
		info.FileVersionLS>>16, info.FileVersionLS&0xffff,
		info.ProductVersionMS>>16, info.ProductVersionMS&0xffff,
		info.ProductVersionLS>>16, info.ProductVersionLS&0xffff), nil
}
