//go:build windows

package security

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TrustedCertThumbprint is the SHA1 thumbprint of the trusted signing certificate.
// Must match TRUSTED_CERT_THUMBPRINT in Driver's signature.h.
const TrustedCertThumbprint = "E723BD5F5C61A0541945A3640F3FEFFE3F090D69"

var (
	modWintrust  = windows.NewLazySystemDLL("wintrust.dll")
	modCrypt32   = windows.NewLazySystemDLL("crypt32.dll")

	procWinVerifyTrust       = modWintrust.NewProc("WinVerifyTrust")
	procCryptQueryObject     = modCrypt32.NewProc("CryptQueryObject")
	procCryptMsgGetParam     = modCrypt32.NewProc("CryptMsgGetParam")
	procCertFreeCertificateContext = modCrypt32.NewProc("CertFreeCertificateContext")
	procCertCloseStore       = modCrypt32.NewProc("CertCloseStore")
	procCryptMsgClose        = modCrypt32.NewProc("CryptMsgClose")
)

const (
	TRUST_E_NOSIGNATURE          = 0x800B0100
	TRUST_E_SUBJECT_NOT_TRUSTED  = 0x800B0004
	TRUST_E_PROVIDER_UNKNOWN     = 0x800B0001
	TRUST_E_BAD_DIGEST           = 0x80096010

	WTD_UI_NONE                = 2
	WTD_REVOKE_NONE            = 0
	WTD_CHOICE_FILE            = 1
	WTD_STATEACTION_VERIFY     = 1
	WTD_STATEACTION_CLOSE      = 2
	WTD_SAFER_FLAG             = 0x100

	CERT_QUERY_OBJECT_FILE              = 1
	CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED = 1 << 10
	CERT_QUERY_FORMAT_FLAG_BINARY       = 1 << 1

	CMSG_SIGNER_INFO_PARAM = 6
	CMSG_SIGNER_CERT_INFO_PARAM = 7
)

// WINTRUST_FILE_INFO
type wintrustFileInfo struct {
	cbStruct       uint32
	pcwszFilePath  *uint16
	hFile          windows.Handle
	pgKnownSubject *windows.GUID
}

// WINTRUST_DATA
type wintrustData struct {
	cbStruct            uint32
	pPolicyCallbackData uintptr
	pSIPClientData      uintptr
	dwUIChoice          uint32
	fdwRevocationChecks uint32
	dwUnionChoice       uint32
	pFile               *wintrustFileInfo
	dwStateAction       uint32
	hWVTStateData       windows.Handle
	pwszURLReference    *uint16
	dwProvFlags         uint32
	dwUIContext         uint32
	pSignatureSettings  uintptr
}

var WINTRUST_ACTION_GENERIC_VERIFY_V2 = windows.GUID{
	Data1: 0xaac56b,
	Data2: 0xcd44,
	Data3: 0x11d0,
	Data4: [8]byte{0x8c, 0xc2, 0x00, 0xc0, 0x4f, 0xc2, 0x95, 0xee},
}

// VerifyAuthenticode uses WinVerifyTrust to verify the Authenticode signature
// of the given file and checks the signer certificate thumbprint.
func VerifyAuthenticode(filePath string) error {
	pathUTF16, err := windows.UTF16PtrFromString(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	fileInfo := wintrustFileInfo{
		cbStruct:      uint32(unsafe.Sizeof(wintrustFileInfo{})),
		pcwszFilePath: pathUTF16,
	}

	trustData := wintrustData{
		cbStruct:            uint32(unsafe.Sizeof(wintrustData{})),
		dwUIChoice:          WTD_UI_NONE,
		fdwRevocationChecks: WTD_REVOKE_NONE,
		dwUnionChoice:       WTD_CHOICE_FILE,
		pFile:               &fileInfo,
		dwStateAction:       WTD_STATEACTION_VERIFY,
		dwProvFlags:         WTD_SAFER_FLAG,
	}

	actionGUID := WINTRUST_ACTION_GENERIC_VERIFY_V2

	r1, _, _ := procWinVerifyTrust.Call(
		^uintptr(0), // INVALID_HANDLE_VALUE
		uintptr(unsafe.Pointer(&actionGUID)),
		uintptr(unsafe.Pointer(&trustData)),
	)

	// Close state regardless of result
	defer func() {
		trustData.dwStateAction = WTD_STATEACTION_CLOSE
		procWinVerifyTrust.Call(
			^uintptr(0),
			uintptr(unsafe.Pointer(&actionGUID)),
			uintptr(unsafe.Pointer(&trustData)),
		)
	}()

	hr := int32(r1)
	if hr != 0 {
		switch uint32(hr) {
		case TRUST_E_NOSIGNATURE:
			return fmt.Errorf("文件未签名")
		case TRUST_E_BAD_DIGEST:
			return fmt.Errorf("文件签名无效(可能被篡改)")
		case TRUST_E_SUBJECT_NOT_TRUSTED:
			return fmt.Errorf("签名证书不受信任")
		default:
			return fmt.Errorf("WinVerifyTrust失败: 0x%08X", uint32(hr))
		}
	}

	// Signature is valid, now verify thumbprint
	return verifyCertThumbprint(filePath)
}

func verifyCertThumbprint(filePath string) error {
	pathUTF16, _ := windows.UTF16PtrFromString(filePath)

	var hMsg windows.Handle
	var hStore windows.Handle
	var encoding uint32

	r1, _, e1 := procCryptQueryObject.Call(
		CERT_QUERY_OBJECT_FILE,
		uintptr(unsafe.Pointer(pathUTF16)),
		CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED,
		CERT_QUERY_FORMAT_FLAG_BINARY,
		0,
		uintptr(unsafe.Pointer(&encoding)),
		0, // content type (not needed)
		0, // format type (not needed)
		uintptr(unsafe.Pointer(&hStore)),
		uintptr(unsafe.Pointer(&hMsg)),
		0,
	)
	if r1 == 0 {
		return fmt.Errorf("CryptQueryObject失败: %v", e1)
	}
	defer procCertCloseStore.Call(uintptr(hStore), 0)
	defer procCryptMsgClose.Call(uintptr(hMsg))

	// Get signer info size
	var signerInfoSize uint32
	r1, _, e1 = procCryptMsgGetParam.Call(
		uintptr(hMsg),
		CMSG_SIGNER_INFO_PARAM,
		0,
		0,
		uintptr(unsafe.Pointer(&signerInfoSize)),
	)
	if r1 == 0 {
		return fmt.Errorf("CryptMsgGetParam(size)失败: %v", e1)
	}

	signerInfoBuf := make([]byte, signerInfoSize)
	r1, _, e1 = procCryptMsgGetParam.Call(
		uintptr(hMsg),
		CMSG_SIGNER_INFO_PARAM,
		0,
		uintptr(unsafe.Pointer(&signerInfoBuf[0])),
		uintptr(unsafe.Pointer(&signerInfoSize)),
	)
	if r1 == 0 {
		return fmt.Errorf("CryptMsgGetParam(data)失败: %v", e1)
	}

	// The CMSG_SIGNER_INFO starts with Issuer and SerialNumber
	// Use CertFindCertificateInStore to find the signer cert,
	// but simpler: enumerate store certs and check thumbprint
	return enumStoreAndCheckThumbprint(hStore)
}

type certContext struct {
	dwCertEncodingType uint32
	pbCertEncoded      *byte
	cbCertEncoded      uint32
	pCertInfo          uintptr
	hCertStore         windows.Handle
}

var procCertEnumCertificatesInStore = modCrypt32.NewProc("CertEnumCertificatesInStore")

func enumStoreAndCheckThumbprint(hStore windows.Handle) error {
	expectedThumbprint := strings.ToUpper(TrustedCertThumbprint)

	var pCtx *certContext
	for {
		r1, _, _ := procCertEnumCertificatesInStore.Call(
			uintptr(hStore),
			uintptr(unsafe.Pointer(pCtx)),
		)
		if r1 == 0 {
			break
		}
		pCtx = (*certContext)(unsafe.Pointer(r1))

		if pCtx.pbCertEncoded != nil && pCtx.cbCertEncoded > 0 {
			certBytes := unsafe.Slice(pCtx.pbCertEncoded, pCtx.cbCertEncoded)
			hash := sha1.Sum(certBytes)
			thumbprint := strings.ToUpper(hex.EncodeToString(hash[:]))

			if thumbprint == expectedThumbprint {
				procCertFreeCertificateContext.Call(uintptr(unsafe.Pointer(pCtx)))
				return nil
			}
		}
	}

	return fmt.Errorf("签名证书指纹不匹配(期望 %s)", expectedThumbprint)
}

// ValidateProcessSignature validates the Authenticode signature of the given PID's executable.
func ValidateProcessSignature(pid uint32) error {
	imagePath, err := processImagePath(pid)
	if err != nil {
		return fmt.Errorf("读取进程路径失败(pid=%d): %w", pid, err)
	}

	if err := VerifyAuthenticode(imagePath); err != nil {
		return fmt.Errorf("进程签名验证失败(pid=%d, path=%s): %w", pid, imagePath, err)
	}

	return nil
}
