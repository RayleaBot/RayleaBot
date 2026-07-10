//go:build windows

package releaseupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const cmsgSignerInfoParam = 6

var (
	crypt32DLL       = windows.NewLazySystemDLL("crypt32.dll")
	cryptMsgGetParam = crypt32DLL.NewProc("CryptMsgGetParam")
	cryptMsgClose    = crypt32DLL.NewProc("CryptMsgClose")
)

type cryptAttributes struct {
	Count      uint32
	Attributes uintptr
}

type cmsgSignerInfo struct {
	Version                 uint32
	Issuer                  windows.CertNameBlob
	SerialNumber            windows.CryptIntegerBlob
	HashAlgorithm           windows.CryptAlgorithmIdentifier
	HashEncryptionAlgorithm windows.CryptAlgorithmIdentifier
	EncryptedHash           windows.CryptDataBlob
	AuthenticatedAttributes cryptAttributes
	UnauthenticatedAttrs    cryptAttributes
}

func VerifyAuthenticodeTree(root, expectedSignerSHA256 string) error {
	if !sha256Pattern.MatchString(expectedSignerSHA256) {
		return errorWithCode(CodeArtifactInvalid, "verify Authenticode signer", errors.New("signed manifest does not contain a valid Windows signer thumbprint"))
	}
	verifiedFiles := 0
	return walkAuthenticodeFiles(root, func(filePath string, requiredSigner bool) error {
		thumbprint, err := verifyAuthenticodeFile(filePath)
		if err != nil {
			return errorWithCode(CodeArtifactInvalid, "verify Authenticode signature", fmt.Errorf("%s: %w", filepath.Base(filePath), err))
		}
		if requiredSigner && !strings.EqualFold(thumbprint, expectedSignerSHA256) {
			return errorWithCode(CodeArtifactInvalid, "verify Authenticode signer", fmt.Errorf("%s signer thumbprint does not match the signed manifest", filepath.Base(filePath)))
		}
		return nil
	}, &verifiedFiles)
}

func VerifyAuthenticodeExecutable(filePath, expectedSignerSHA256 string) error {
	if !sha256Pattern.MatchString(expectedSignerSHA256) {
		return errorWithCode(CodeArtifactInvalid, "verify Authenticode signer", errors.New("signed manifest does not contain a valid Windows signer thumbprint"))
	}
	thumbprint, err := verifyAuthenticodeFile(filePath)
	if err != nil {
		return errorWithCode(CodeArtifactInvalid, "verify Authenticode signature", fmt.Errorf("%s: %w", filepath.Base(filePath), err))
	}
	if !strings.EqualFold(thumbprint, expectedSignerSHA256) {
		return errorWithCode(CodeArtifactInvalid, "verify Authenticode signer", fmt.Errorf("%s signer thumbprint does not match the signed manifest", filepath.Base(filePath)))
	}
	return nil
}

func verifyAuthenticodeFile(filePath string) (string, error) {
	filePathUTF16, err := windows.UTF16PtrFromString(filePath)
	if err != nil {
		return "", err
	}
	fileInfo := &windows.WinTrustFileInfo{
		Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
		FilePath: filePathUTF16,
	}
	trustData := &windows.WinTrustData{
		Size:                            uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:                        windows.WTD_UI_NONE,
		RevocationChecks:                windows.WTD_REVOKE_NONE,
		UnionChoice:                     windows.WTD_CHOICE_FILE,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(fileInfo),
		StateAction:                     windows.WTD_STATEACTION_VERIFY,
		ProvFlags:                       windows.WTD_REVOCATION_CHECK_NONE,
		UIContext:                       windows.WTD_UICONTEXT_INSTALL,
	}
	verifyErr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, trustData)
	trustData.StateAction = windows.WTD_STATEACTION_CLOSE
	closeErr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, trustData)
	if verifyErr != nil {
		return "", verifyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return signerCertificateSHA256(filePathUTF16)
}

func signerCertificateSHA256(filePathUTF16 *uint16) (string, error) {
	var encodingType, contentType, formatType uint32
	var store, message windows.Handle
	if err := windows.CryptQueryObject(
		windows.CERT_QUERY_OBJECT_FILE,
		unsafe.Pointer(filePathUTF16),
		windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED,
		windows.CERT_QUERY_FORMAT_FLAG_BINARY,
		0,
		&encodingType,
		&contentType,
		&formatType,
		&store,
		&message,
		nil,
	); err != nil {
		return "", err
	}
	defer windows.CertCloseStore(store, 0)
	defer cryptMsgClose.Call(uintptr(message))

	var signerInfoSize uint32
	result, _, callErr := cryptMsgGetParam.Call(uintptr(message), cmsgSignerInfoParam, 0, 0, uintptr(unsafe.Pointer(&signerInfoSize)))
	if result == 0 || signerInfoSize == 0 {
		return "", fmt.Errorf("query signer info size: %w", callErr)
	}
	buffer := make([]byte, signerInfoSize)
	result, _, callErr = cryptMsgGetParam.Call(uintptr(message), cmsgSignerInfoParam, 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&signerInfoSize)))
	if result == 0 {
		return "", fmt.Errorf("query signer info: %w", callErr)
	}
	signerInfo := (*cmsgSignerInfo)(unsafe.Pointer(&buffer[0]))
	certificateInfo := windows.CertInfo{
		Issuer:       signerInfo.Issuer,
		SerialNumber: signerInfo.SerialNumber,
	}
	certificate, err := windows.CertFindCertificateInStore(
		store,
		windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING,
		0,
		windows.CERT_FIND_SUBJECT_CERT,
		unsafe.Pointer(&certificateInfo),
		nil,
	)
	if err != nil {
		return "", err
	}
	defer windows.CertFreeCertificateContext(certificate)
	if certificate.Length == 0 || certificate.EncodedCert == nil {
		return "", errors.New("signer certificate is empty")
	}
	encodedCertificate := unsafe.Slice(certificate.EncodedCert, certificate.Length)
	digest := sha256.Sum256(encodedCertificate)
	return hex.EncodeToString(digest[:]), nil
}
