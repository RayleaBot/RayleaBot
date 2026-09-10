//go:build !windows

package releaseupdate

import "errors"

func VerifyAuthenticodeTree(root, expectedSignerSHA256 string) error {
	return errorWithCode(CodeUpdateNotSupported, "verify Authenticode", errors.New("only Windows supports Authenticode verification"))
}

func VerifyAuthenticodeExecutable(filePath, expectedSignerSHA256 string) error {
	return errorWithCode(CodeUpdateNotSupported, "verify Authenticode", errors.New("only Windows supports Authenticode verification"))
}
