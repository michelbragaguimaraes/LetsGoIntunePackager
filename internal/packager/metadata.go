package packager

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
)

const (
	// ToolVersion matches Microsoft IntuneWinAppUtil version
	ToolVersion = "1.8.6.0"
	// ProfileIdentifier is the encryption profile identifier
	ProfileIdentifier = "ProfileVersion1"
	// FileDigestAlgorithm is the hash algorithm used
	FileDigestAlgorithm = "SHA256"
)

// ApplicationInfo is the root XML element for Detection.xml
// Field order matches official Microsoft IntuneWinAppUtil output
type ApplicationInfo struct {
	XMLName                xml.Name        `xml:"ApplicationInfo"`
	XSD                    string          `xml:"xmlns:xsd,attr"`
	XSI                    string          `xml:"xmlns:xsi,attr"`
	ToolVersion            string          `xml:"ToolVersion,attr"`
	Name                   string          `xml:"Name"`
	UnencryptedContentSize int64           `xml:"UnencryptedContentSize"`
	FileName               string          `xml:"FileName"`
	SetupFile              string          `xml:"SetupFile"`
	EncryptionInfo         EncryptionXML   `xml:"EncryptionInfo"`
	MsiInfo                *MsiInfoXML     `xml:"MsiInfo,omitempty"`
}

// EncryptionXML contains the encryption metadata in XML format
type EncryptionXML struct {
	EncryptionKey        string `xml:"EncryptionKey"`
	MacKey               string `xml:"MacKey"`
	InitializationVector string `xml:"InitializationVector"`
	Mac                  string `xml:"Mac"`
	ProfileIdentifier    string `xml:"ProfileIdentifier"`
	FileDigest           string `xml:"FileDigest"`
	FileDigestAlgorithm  string `xml:"FileDigestAlgorithm"`
}

// MsiInfoXML contains MSI-specific metadata (only for .msi files)
// Field order matches official Microsoft IntuneWinAppUtil output
type MsiInfoXML struct {
	MsiProductCode                string `xml:"MsiProductCode,omitempty"`
	MsiProductVersion             string `xml:"MsiProductVersion,omitempty"`
	MsiPackageCode                string `xml:"MsiPackageCode,omitempty"`
	MsiUpgradeCode                string `xml:"MsiUpgradeCode,omitempty"`
	MsiExecutionContext           string `xml:"MsiExecutionContext,omitempty"`
	MsiRequiresLogon              bool   `xml:"MsiRequiresLogon"`
	MsiRequiresReboot             bool   `xml:"MsiRequiresReboot"`
	MsiIsMachineInstall           bool   `xml:"MsiIsMachineInstall"`
	MsiIsUserInstall              bool   `xml:"MsiIsUserInstall"`
	MsiIncludesServices           bool   `xml:"MsiIncludesServices"`
	MsiIncludesODBCDataSource     bool   `xml:"MsiIncludesODBCDataSource"`
	MsiContainsSystemRegistryKeys bool   `xml:"MsiContainsSystemRegistryKeys"`
	MsiContainsSystemFolders      bool   `xml:"MsiContainsSystemFolders"`
	MsiPublisher                  string `xml:"MsiPublisher,omitempty"`
}

// MetadataParams holds parameters for generating Detection.xml
type MetadataParams struct {
	// Name is the application name (derived from setup file name)
	Name string
	// SetupFile is the setup file name (e.g., "setup.msi")
	SetupFile string
	// UnencryptedContentSize is the size of the ZIP before encryption
	UnencryptedContentSize int64
	// EncryptionInfo contains encryption keys and hashes
	EncryptionInfo *EncryptionInfo
	// MsiInfo contains MSI metadata (optional, only for .msi files)
	MsiInfo *MsiInfo
}

// GenerateDetectionXML creates the Detection.xml content
func GenerateDetectionXML(params *MetadataParams) ([]byte, error) {
	if params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if params.EncryptionInfo == nil {
		return nil, fmt.Errorf("encryption info cannot be nil")
	}

	// Create encryption info XML with base64-encoded values
	encXML := EncryptionXML{
		EncryptionKey:        base64.StdEncoding.EncodeToString(params.EncryptionInfo.EncryptionKey),
		MacKey:               base64.StdEncoding.EncodeToString(params.EncryptionInfo.MacKey),
		InitializationVector: base64.StdEncoding.EncodeToString(params.EncryptionInfo.InitializationVector),
		Mac:                  base64.StdEncoding.EncodeToString(params.EncryptionInfo.Mac),
		ProfileIdentifier:    ProfileIdentifier,
		FileDigest:           base64.StdEncoding.EncodeToString(params.EncryptionInfo.FileDigest),
		FileDigestAlgorithm:  FileDigestAlgorithm,
	}

	// Create application info
	appInfo := ApplicationInfo{
		XSD:                    "http://www.w3.org/2001/XMLSchema",
		XSI:                    "http://www.w3.org/2001/XMLSchema-instance",
		ToolVersion:            ToolVersion,
		Name:                   params.Name,
		SetupFile:              params.SetupFile,
		FileName:               "IntunePackage.intunewin",
		UnencryptedContentSize: params.UnencryptedContentSize,
		EncryptionInfo:         encXML,
	}

	// Add MSI info if available
	if params.MsiInfo != nil {
		// Use ProductName from MSI if available (overrides filename-based name)
		if params.MsiInfo.ProductName != "" {
			appInfo.Name = params.MsiInfo.ProductName
		}

		appInfo.MsiInfo = &MsiInfoXML{
			MsiProductCode:                params.MsiInfo.ProductCode,
			MsiProductVersion:             params.MsiInfo.ProductVersion,
			MsiPackageCode:                params.MsiInfo.PackageCode,
			MsiUpgradeCode:                params.MsiInfo.UpgradeCode,
			MsiExecutionContext:           "Any",
			MsiRequiresLogon:              false,
			MsiRequiresReboot:             false,
			MsiIsMachineInstall:           true,
			MsiIsUserInstall:              false,
			MsiIncludesServices:           false,
			MsiIncludesODBCDataSource:     false,
			MsiContainsSystemRegistryKeys: false,
			MsiContainsSystemFolders:      false,
			MsiPublisher:                  params.MsiInfo.Publisher,
		}
	}

	// Generate XML without declaration (Microsoft's official tool doesn't include it)
	xmlData, err := xml.MarshalIndent(appInfo, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal XML: %w", err)
	}

	// Convert LF to CRLF line endings for Windows/Intune compatibility
	// Microsoft's official IntuneWinAppUtil uses CRLF line endings
	result := bytes.ReplaceAll(xmlData, []byte("\n"), []byte("\r\n"))

	return result, nil
}

// GetApplicationName extracts the application name from the setup file
func GetApplicationName(setupFile string) string {
	// Remove extension to get base name
	name := setupFile
	for _, ext := range []string{".msi", ".exe", ".MSI", ".EXE"} {
		if len(name) > len(ext) && name[len(name)-len(ext):] == ext {
			name = name[:len(name)-len(ext)]
			break
		}
	}
	return name
}
