package packager

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestGenerateDetectionXML(t *testing.T) {
	encInfo := &EncryptionInfo{
		EncryptionKey:        []byte("test-encryption-key-32bytes!!!!"),
		MacKey:               []byte("test-mac-key-32bytes!!!!!!!!!!"),
		InitializationVector: []byte("test-iv-16bytes!"),
		Mac:                  []byte("test-mac-32bytes!!!!!!!!!!!!!"),
		FileDigest:           []byte("test-digest-32bytes!!!!!!!!!!"),
	}

	params := &MetadataParams{
		Name:                   "TestApp",
		SetupFile:              "setup.exe",
		UnencryptedContentSize: 12345,
		EncryptionInfo:         encInfo,
		MsiInfo:                nil,
	}

	xmlData, err := GenerateDetectionXML(params)
	if err != nil {
		t.Fatalf("GenerateDetectionXML() error = %v", err)
	}

	// Verify it's valid XML
	var appInfo ApplicationInfo
	if err := xml.Unmarshal(xmlData, &appInfo); err != nil {
		t.Fatalf("Generated XML is invalid: %v", err)
	}

	// Verify content
	if appInfo.Name != "TestApp" {
		t.Errorf("Name = %s, want TestApp", appInfo.Name)
	}
	if appInfo.SetupFile != "setup.exe" {
		t.Errorf("SetupFile = %s, want setup.exe", appInfo.SetupFile)
	}
	if appInfo.UnencryptedContentSize != 12345 {
		t.Errorf("UnencryptedContentSize = %d, want 12345", appInfo.UnencryptedContentSize)
	}
	if appInfo.FileName != "IntunePackage.intunewin" {
		t.Errorf("FileName = %s, want IntunePackage.intunewin", appInfo.FileName)
	}
	if appInfo.ToolVersion != "1.8.6.0" {
		t.Errorf("ToolVersion = %s, want 1.8.6.0", appInfo.ToolVersion)
	}

	// Verify encryption info
	if appInfo.EncryptionInfo.FileDigestAlgorithm != "SHA256" {
		t.Errorf("FileDigestAlgorithm = %s, want SHA256", appInfo.EncryptionInfo.FileDigestAlgorithm)
	}
	if appInfo.EncryptionInfo.ProfileIdentifier != "ProfileVersion1" {
		t.Errorf("ProfileIdentifier = %s, want ProfileVersion1", appInfo.EncryptionInfo.ProfileIdentifier)
	}
}

func TestGenerateDetectionXMLWithMsiInfo(t *testing.T) {
	encInfo := &EncryptionInfo{
		EncryptionKey:        []byte("test-encryption-key-32bytes!!!!"),
		MacKey:               []byte("test-mac-key-32bytes!!!!!!!!!!"),
		InitializationVector: []byte("test-iv-16bytes!"),
		Mac:                  []byte("test-mac-32bytes!!!!!!!!!!!!!"),
		FileDigest:           []byte("test-digest-32bytes!!!!!!!!!!"),
	}

	msiInfo := &MsiInfo{
		ProductCode:    "{12345678-1234-1234-1234-123456789ABC}",
		ProductVersion: "1.0.0.0",
		PackageCode:    "{ABCDEFGH-ABCD-ABCD-ABCD-ABCDEFGHIJKL}",
		Publisher:      "Test Publisher",
		UpgradeCode:    "{87654321-4321-4321-4321-CBA987654321}",
	}

	params := &MetadataParams{
		Name:                   "TestMSI",
		SetupFile:              "setup.msi",
		UnencryptedContentSize: 54321,
		EncryptionInfo:         encInfo,
		MsiInfo:                msiInfo,
	}

	xmlData, err := GenerateDetectionXML(params)
	if err != nil {
		t.Fatalf("GenerateDetectionXML() error = %v", err)
	}

	// Verify it's valid XML
	var appInfo ApplicationInfo
	if err := xml.Unmarshal(xmlData, &appInfo); err != nil {
		t.Fatalf("Generated XML is invalid: %v", err)
	}

	// Verify MSI info is included
	if appInfo.MsiInfo == nil {
		t.Fatal("MsiInfo is nil")
	}
	if appInfo.MsiInfo.MsiProductCode != msiInfo.ProductCode {
		t.Errorf("MsiProductCode = %s, want %s", appInfo.MsiInfo.MsiProductCode, msiInfo.ProductCode)
	}
	if appInfo.MsiInfo.MsiProductVersion != msiInfo.ProductVersion {
		t.Errorf("MsiProductVersion = %s, want %s", appInfo.MsiInfo.MsiProductVersion, msiInfo.ProductVersion)
	}
	if appInfo.MsiInfo.MsiPublisher != msiInfo.Publisher {
		t.Errorf("MsiPublisher = %s, want %s", appInfo.MsiInfo.MsiPublisher, msiInfo.Publisher)
	}
}

func TestGenerateDetectionXMLFormat(t *testing.T) {
	encInfo := &EncryptionInfo{
		EncryptionKey:        []byte("test-encryption-key-32bytes!!!!"),
		MacKey:               []byte("test-mac-key-32bytes!!!!!!!!!!"),
		InitializationVector: []byte("test-iv-16bytes!"),
		Mac:                  []byte("test-mac-32bytes!!!!!!!!!!!!!"),
		FileDigest:           []byte("test-digest-32bytes!!!!!!!!!!"),
	}

	params := &MetadataParams{
		Name:                   "Test",
		SetupFile:              "test.exe",
		UnencryptedContentSize: 1000,
		EncryptionInfo:         encInfo,
		MsiInfo:                nil,
	}

	xmlData, err := GenerateDetectionXML(params)
	if err != nil {
		t.Fatalf("GenerateDetectionXML() error = %v", err)
	}

	xmlStr := string(xmlData)

	// Check XML declaration (Go's xml.Header uses UTF-8 uppercase)
	if !strings.HasPrefix(xmlStr, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Error("Missing or incorrect XML declaration")
	}

	// Check namespace attributes
	if !strings.Contains(xmlStr, "xmlns:xsd=\"http://www.w3.org/2001/XMLSchema\"") {
		t.Error("Missing xsd namespace")
	}
	if !strings.Contains(xmlStr, "xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"") {
		t.Error("Missing xsi namespace")
	}

	// Check ToolVersion attribute
	if !strings.Contains(xmlStr, "ToolVersion=\"1.8.6.0\"") {
		t.Error("Missing or incorrect ToolVersion attribute")
	}
}

func TestGenerateDetectionXMLNilParams(t *testing.T) {
	_, err := GenerateDetectionXML(nil)
	if err == nil {
		t.Error("Expected error for nil params")
	}
}

func TestGenerateDetectionXMLNilEncryptionInfo(t *testing.T) {
	params := &MetadataParams{
		Name:                   "Test",
		SetupFile:              "test.exe",
		UnencryptedContentSize: 1000,
		EncryptionInfo:         nil,
	}
	_, err := GenerateDetectionXML(params)
	if err == nil {
		t.Error("Expected error for nil encryption info")
	}
}
