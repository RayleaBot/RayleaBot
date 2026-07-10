//go:build !windows

package releaseupdate

import "errors"

func VerifyAuthenticodeTree(root, expectedSignerSHA256 string) error {
	return errorWithCode(CodeUpdateNotSupported, "verify Authenticode", errors.New("Authenticode verification is available only on Windows"))
}

func VerifyAuthenticodeExecutable(filePath, expectedSignerSHA256 string) error {
	return errorWithCode(CodeUpdateNotSupported, "verify Authenticode", errors.New("Authenticode verification is available only on Windows"))
}
