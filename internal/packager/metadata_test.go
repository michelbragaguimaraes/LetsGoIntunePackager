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

	// Microsoft's tool serializes with .NET's XmlSerializer, which writes a
	// lowercase "utf-8" and a CRLF before the root element.
	if !strings.HasPrefix(xmlStr, "<?xml version=\"1.0\" encoding=\"utf-8\"?>\r\n") {
		t.Errorf("Missing or incorrect XML declaration, got %.60q", xmlStr)
	}

	// Description sits between Name and UnencryptedContentSize in the official
	// layout, and is emitted even when empty.
	if !strings.Contains(xmlStr, "<Description></Description>") {
		t.Error("Missing Description element")
	}
	if strings.Index(xmlStr, "<Description>") < strings.Index(xmlStr, "<Name>") {
		t.Error("Description must follow Name")
	}
	if strings.Index(xmlStr, "<Description>") > strings.Index(xmlStr, "<UnencryptedContentSize>") {
		t.Error("Description must precede UnencryptedContentSize")
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

func TestGetApplicationName(t *testing.T) {
	tests := []struct {
		setupFile string
		want      string
	}{
		{"setup.msi", "setup"},
		{"install.exe", "install"},
		{"SETUP.MSI", "SETUP"},
		// Every extension is stripped, not just .msi and .exe.
		{"Install-App.ps1", "Install-App"},
		{"deploy.cmd", "deploy"},
		{"deploy.bat", "deploy"},
		// A directory component must not leak into the output filename, or the
		// write fails on a path that does not exist under the output folder.
		{"nested/Install-App.ps1", "Install-App"},
		{`nested\setup.exe`, "setup"},
		{`a/b\c/setup.msi`, "setup"},
		// Dots inside the name are preserved.
		{"app.v1.2.exe", "app.v1.2"},
		{"noextension", "noextension"},
	}

	for _, tt := range tests {
		t.Run(tt.setupFile, func(t *testing.T) {
			if got := GetApplicationName(tt.setupFile); got != tt.want {
				t.Errorf("GetApplicationName(%q) = %q, want %q", tt.setupFile, got, tt.want)
			}
		})
	}
}

func TestGenerateDetectionXMLUsesDerivedMsiFields(t *testing.T) {
	encInfo := &EncryptionInfo{
		EncryptionKey:        make([]byte, 32),
		MacKey:               make([]byte, 32),
		InitializationVector: make([]byte, 16),
		Mac:                  make([]byte, 32),
		FileDigest:           make([]byte, 32),
	}

	params := &MetadataParams{
		Name:           "ignored",
		SetupFile:      "setup.msi",
		EncryptionInfo: encInfo,
		MsiInfo: &MsiInfo{
			ProductName:            "Contoso App",
			UpgradeCode:            "{60A16214-D37D-D2BA-C838-3B93DEAC2F7F}",
			ExecutionContext:       "System",
			IsMachineInstall:       true,
			IsUserInstall:          false,
			IncludesServices:       true,
			IncludesODBCDataSource: true,
		},
	}

	xmlStr := string(mustXML(t, params))

	for _, want := range []string{
		"<Name>Contoso App</Name>",
		"<MsiUpgradeCode>{60A16214-D37D-D2BA-C838-3B93DEAC2F7F}</MsiUpgradeCode>",
		"<MsiExecutionContext>System</MsiExecutionContext>",
		"<MsiIsMachineInstall>true</MsiIsMachineInstall>",
		"<MsiIsUserInstall>false</MsiIsUserInstall>",
		"<MsiIncludesServices>true</MsiIncludesServices>",
		"<MsiIncludesODBCDataSource>true</MsiIncludesODBCDataSource>",
	} {
		if !strings.Contains(xmlStr, want) {
			t.Errorf("Detection.xml missing %s\ngot:\n%s", want, xmlStr)
		}
	}
}

func TestGenerateDetectionXMLPerUserMsi(t *testing.T) {
	params := &MetadataParams{
		SetupFile: "setup.msi",
		EncryptionInfo: &EncryptionInfo{
			EncryptionKey:        make([]byte, 32),
			MacKey:               make([]byte, 32),
			InitializationVector: make([]byte, 16),
			Mac:                  make([]byte, 32),
			FileDigest:           make([]byte, 32),
		},
		MsiInfo: &MsiInfo{ExecutionContext: "User", IsUserInstall: true},
	}

	xmlStr := string(mustXML(t, params))
	if !strings.Contains(xmlStr, "<MsiIsUserInstall>true</MsiIsUserInstall>") ||
		!strings.Contains(xmlStr, "<MsiIsMachineInstall>false</MsiIsMachineInstall>") {
		t.Errorf("per-user MSI not reflected in Detection.xml:\n%s", xmlStr)
	}
}

func mustXML(t *testing.T, params *MetadataParams) []byte {
	t.Helper()
	data, err := GenerateDetectionXML(params)
	if err != nil {
		t.Fatalf("GenerateDetectionXML() error = %v", err)
	}
	return data
}
