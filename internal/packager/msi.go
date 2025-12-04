package packager

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf16"

	"github.com/richardlehane/mscfb"
	"github.com/richardlehane/msoleps"
)

// MsiInfo contains metadata extracted from an MSI file
type MsiInfo struct {
	ProductCode    string // {GUID} from Property table
	ProductVersion string // Version from Property table
	PackageCode    string // {GUID} from Summary Information
	Publisher      string // Manufacturer from Property table
	UpgradeCode    string // {GUID} from Property table
	ProductName    string // ProductName from Property table (for display)
}

// IsMsiFile checks if the given file path has an .msi extension
func IsMsiFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".msi")
}

// ExtractMsiInfo extracts metadata from an MSI file using OLE/CFB parsing
func ExtractMsiInfo(msiPath string) (*MsiInfo, error) {
	file, err := os.Open(msiPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open MSI file: %w", err)
	}
	defer file.Close()

	// Parse the OLE Compound File
	doc, err := mscfb.New(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MSI as OLE document: %w", err)
	}

	info := &MsiInfo{}
	var stringPool []string

	// First pass: collect data from streams
	for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
		name := entry.Name

		// Summary Information stream contains PackageCode (PIDSI_REVNUMBER)
		if name == "\x05SummaryInformation" {
			data, readErr := io.ReadAll(entry)
			if readErr == nil {
				info.PackageCode = extractPackageCodeFromOLEPS(data)
			}
		}

		// The MSI string pool - decode it to get property values
		if name == "!_StringData" {
			data, readErr := io.ReadAll(entry)
			if readErr == nil {
				stringPool = decodeStringPool(data)
			}
		}
	}

	// Reset file and read raw data for pattern matching fallback
	file.Seek(0, 0)
	rawData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read MSI file: %w", err)
	}

	// Try direct pattern matching in raw data first (most reliable for MSI)
	// MSI stores properties as contiguous strings like: "ProductCode{GUID}ProductVersion1.0.0"
	info.ProductCode = extractMsiPropertyValue(rawData, "ProductCode")
	info.ProductVersion = extractMsiPropertyValue(rawData, "ProductVersion")
	info.Publisher = extractMsiPropertyValue(rawData, "Manufacturer")
	info.UpgradeCode = extractMsiPropertyValue(rawData, "UpgradeCode")
	info.ProductName = extractMsiPropertyValue(rawData, "ProductName")

	// Fallback to string pool search if direct extraction failed
	if len(stringPool) > 0 {
		if info.ProductCode == "" {
			info.ProductCode = findInStringPool(stringPool, isValidGUID)
		}
		if info.ProductVersion == "" {
			info.ProductVersion = findVersionInStringPool(stringPool)
		}
		if info.Publisher == "" {
			info.Publisher = findPublisherInStringPool(stringPool)
		}
		if info.UpgradeCode == "" {
			info.UpgradeCode = findSecondGUIDInStringPool(stringPool, info.ProductCode)
		}
	}

	// Last resort fallback with different search methods
	if info.ProductCode == "" {
		info.ProductCode = extractGUIDNearProperty(rawData, "ProductCode")
	}
	if info.ProductVersion == "" {
		info.ProductVersion = extractVersionNearProperty(rawData, "ProductVersion")
	}
	if info.Publisher == "" {
		info.Publisher = extractStringNearProperty(rawData, "Manufacturer")
	}
	if info.UpgradeCode == "" {
		info.UpgradeCode = extractGUIDNearProperty(rawData, "UpgradeCode")
	}

	// If we still don't have PackageCode, search in raw data
	if info.PackageCode == "" {
		info.PackageCode = findFirstGUIDInData(rawData)
	}

	return info, nil
}

// extractPackageCodeFromOLEPS extracts the PackageCode from OLE Property Set Summary Information
func extractPackageCodeFromOLEPS(data []byte) string {
	// Try to parse as OLE Property Set using NewFrom
	reader := bytes.NewReader(data)
	props, err := msoleps.NewFrom(reader)
	if err == nil {
		// PIDSI_REVNUMBER is property ID 9 in Summary Information
		for _, prop := range props.Property {
			if prop.Name == "PIDSI_REVNUMBER" || prop.Name == "Revision Number" {
				str := fmt.Sprintf("%v", prop)
				// MSI PackageCode is stored in a special compressed format
				guid := decompressMSIGUID(str)
				if guid != "" {
					return guid
				}
				// Or it might be a regular GUID
				if isValidGUID(str) {
					return str
				}
			}
		}
	}

	// Fallback: look for GUID pattern in raw data
	for i := 0; i < len(data)-38; i++ {
		if data[i] == '{' {
			guid := extractGUIDAt(data, i)
			if guid != "" {
				return guid
			}
		}
	}

	// Try to find compressed GUID
	return findCompressedGUID(data)
}

// decompressMSIGUID converts MSI's compressed GUID format to standard GUID format
// MSI uses a special format where each group of the GUID is reversed
func decompressMSIGUID(compressed string) string {
	// Remove any braces or dashes first
	clean := strings.ReplaceAll(compressed, "{", "")
	clean = strings.ReplaceAll(clean, "}", "")
	clean = strings.ReplaceAll(clean, "-", "")

	if len(clean) != 32 {
		return ""
	}

	// MSI compressed format: each byte pair is swapped within groups
	// Group 1 (8 chars): reverse pairs -> 12345678 becomes 21436587
	// Group 2 (4 chars): reverse pairs -> 1234 becomes 2143
	// Group 3 (4 chars): reverse pairs -> 1234 becomes 2143
	// Group 4 (4 chars): no change
	// Group 5 (12 chars): no change

	var result strings.Builder
	result.WriteString("{")

	// Group 1: 8 characters, swap pairs
	for i := 0; i < 8; i += 2 {
		result.WriteByte(clean[i+1])
		result.WriteByte(clean[i])
	}
	result.WriteString("-")

	// Group 2: 4 characters, swap pairs
	for i := 8; i < 12; i += 2 {
		result.WriteByte(clean[i+1])
		result.WriteByte(clean[i])
	}
	result.WriteString("-")

	// Group 3: 4 characters, swap pairs
	for i := 12; i < 16; i += 2 {
		result.WriteByte(clean[i+1])
		result.WriteByte(clean[i])
	}
	result.WriteString("-")

	// Group 4: 4 characters, swap pairs
	for i := 16; i < 20; i += 2 {
		result.WriteByte(clean[i+1])
		result.WriteByte(clean[i])
	}
	result.WriteString("-")

	// Group 5: 12 characters, swap pairs
	for i := 20; i < 32; i += 2 {
		result.WriteByte(clean[i+1])
		result.WriteByte(clean[i])
	}

	result.WriteString("}")

	guid := result.String()
	if isValidGUID(guid) {
		return guid
	}
	return ""
}

// findCompressedGUID searches for MSI's compressed GUID format in data
func findCompressedGUID(data []byte) string {
	// Look for 32-character hex strings that could be compressed GUIDs
	for i := 0; i < len(data)-32; i++ {
		if isHexChar(data[i]) {
			potential := string(data[i : i+32])
			if isAllHex(potential) {
				guid := decompressMSIGUID(potential)
				if guid != "" {
					return guid
				}
			}
		}
	}
	return ""
}

// decodeStringPool decodes the MSI string pool
func decodeStringPool(data []byte) []string {
	var strings []string

	// MSI string pool is a series of null-terminated strings
	// or length-prefixed strings depending on the format

	// Try null-terminated UTF-16LE strings
	var current []rune
	for i := 0; i+1 < len(data); i += 2 {
		char := binary.LittleEndian.Uint16(data[i:])
		if char == 0 {
			if len(current) > 0 {
				strings = append(strings, string(current))
				current = nil
			}
		} else if char >= 32 && char < 65535 {
			current = append(current, rune(char))
		}
	}
	if len(current) > 0 {
		strings = append(strings, string(current))
	}

	return strings
}

// findInStringPool searches for a string matching the predicate in the string pool
func findInStringPool(pool []string, predicate func(string) bool) string {
	for _, s := range pool {
		if predicate(s) {
			return s
		}
	}
	return ""
}

// findVersionInStringPool finds a version string in the string pool
func findVersionInStringPool(pool []string) string {
	for _, s := range pool {
		if isValidVersion(s) {
			return s
		}
	}
	return ""
}

// findPublisherInStringPool finds a likely publisher name in the string pool
func findPublisherInStringPool(pool []string) string {
	// Look for strings that look like company names (contain spaces, reasonable length)
	for _, s := range pool {
		if len(s) >= 3 && len(s) <= 100 {
			// Skip GUIDs and versions
			if isValidGUID(s) || isValidVersion(s) {
				continue
			}
			// Look for strings with spaces (company names usually have them)
			if strings.Contains(s, " ") && !strings.HasPrefix(s, "{") {
				// Additional heuristics: should start with uppercase letter
				if len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z' {
					return s
				}
			}
		}
	}
	return ""
}

// findSecondGUIDInStringPool finds a GUID that's different from the first one
func findSecondGUIDInStringPool(pool []string, firstGUID string) string {
	for _, s := range pool {
		if isValidGUID(s) && s != firstGUID {
			return s
		}
	}
	return ""
}

// extractGUIDNearProperty searches for a GUID near a property name
func extractGUIDNearProperty(data []byte, propertyName string) string {
	// Search in UTF-16LE
	propBytes := stringToUTF16LE(propertyName)
	for i := 0; i < len(data)-len(propBytes); i++ {
		if bytes.Equal(data[i:i+len(propBytes)], propBytes) {
			// Search forward for a GUID
			searchEnd := min(i+len(propBytes)+2048, len(data))
			for j := i + len(propBytes); j < searchEnd-38; j++ {
				if data[j] == '{' {
					guid := extractGUIDAt(data, j)
					if guid != "" {
						return guid
					}
				}
			}
		}
	}

	// Search in ASCII
	for i := 0; i < len(data)-len(propertyName); i++ {
		if string(data[i:i+len(propertyName)]) == propertyName {
			searchEnd := min(i+len(propertyName)+1024, len(data))
			for j := i + len(propertyName); j < searchEnd-38; j++ {
				if data[j] == '{' {
					guid := extractGUIDAt(data, j)
					if guid != "" {
						return guid
					}
				}
			}
		}
	}

	return ""
}

// extractVersionNearProperty searches for a version near a property name
func extractVersionNearProperty(data []byte, propertyName string) string {
	propBytes := stringToUTF16LE(propertyName)
	for i := 0; i < len(data)-len(propBytes); i++ {
		if bytes.Equal(data[i:i+len(propBytes)], propBytes) {
			searchStart := i + len(propBytes)
			searchEnd := min(searchStart+512, len(data))

			// Look for UTF-16LE version
			for j := searchStart; j < searchEnd-10; j += 2 {
				if j+2 <= len(data) {
					char := binary.LittleEndian.Uint16(data[j:])
					if char >= '0' && char <= '9' {
						version := extractUTF16Version(data[j:])
						if isValidVersion(version) {
							return version
						}
					}
				}
			}
		}
	}
	return ""
}

// extractStringNearProperty searches for a string near a property name
func extractStringNearProperty(data []byte, propertyName string) string {
	propBytes := stringToUTF16LE(propertyName)
	for i := 0; i < len(data)-len(propBytes); i++ {
		if bytes.Equal(data[i:i+len(propBytes)], propBytes) {
			searchStart := i + len(propBytes)
			// Skip some bytes (there's usually some padding)
			for offset := 0; offset < 64; offset += 2 {
				str := extractUTF16StringClean(data[searchStart+offset:])
				if len(str) >= 3 && len(str) <= 128 {
					// Validate it looks like a publisher name
					if !isValidGUID(str) && !isValidVersion(str) {
						return str
					}
				}
			}
		}
	}
	return ""
}

// findFirstGUIDInData finds the first valid GUID in the data
func findFirstGUIDInData(data []byte) string {
	for i := 0; i < len(data)-38; i++ {
		if data[i] == '{' {
			guid := extractGUIDAt(data, i)
			if guid != "" {
				return guid
			}
		}
	}
	return ""
}

// extractGUIDAt extracts a GUID starting at the given position
func extractGUIDAt(data []byte, pos int) string {
	if pos+38 > len(data) {
		return ""
	}

	potential := string(data[pos : pos+38])
	if isValidGUID(potential) {
		return potential
	}
	return ""
}

// extractUTF16Version extracts a version string from UTF-16LE data
func extractUTF16Version(data []byte) string {
	var chars []rune
	for i := 0; i+1 < len(data) && len(chars) < 32; i += 2 {
		char := binary.LittleEndian.Uint16(data[i:])
		if char == 0 {
			break
		}
		if (char >= '0' && char <= '9') || char == '.' {
			chars = append(chars, rune(char))
		} else {
			break
		}
	}
	return string(chars)
}

// isValidVersion checks if a string looks like a valid version
func isValidVersion(s string) bool {
	if len(s) < 3 || len(s) > 32 {
		return false
	}
	if !strings.Contains(s, ".") {
		return false
	}
	if s[0] < '0' || s[0] > '9' {
		return false
	}
	if s[len(s)-1] < '0' || s[len(s)-1] > '9' {
		return false
	}
	dotCount := strings.Count(s, ".")
	return dotCount >= 1 && dotCount <= 3
}

// extractUTF16StringClean extracts a clean UTF-16LE string
func extractUTF16StringClean(data []byte) string {
	var runes []rune
	for i := 0; i+1 < len(data) && len(runes) < 128; i += 2 {
		char := binary.LittleEndian.Uint16(data[i:])
		if char == 0 {
			if len(runes) > 0 {
				break
			}
			continue
		}
		if char >= 32 && char < 127 {
			runes = append(runes, rune(char))
		} else if len(runes) > 0 {
			break
		}
	}
	return strings.TrimSpace(string(runes))
}

// stringToUTF16LE converts a string to UTF-16LE bytes
func stringToUTF16LE(s string) []byte {
	runes := []rune(s)
	u16 := utf16.Encode(runes)

	result := make([]byte, len(u16)*2)
	for i, r := range u16 {
		binary.LittleEndian.PutUint16(result[i*2:], r)
	}

	return result
}

// isValidGUID checks if a string is a valid GUID format
func isValidGUID(s string) bool {
	if len(s) != 38 {
		return false
	}
	if s[0] != '{' || s[37] != '}' {
		return false
	}
	if s[9] != '-' || s[14] != '-' || s[19] != '-' || s[24] != '-' {
		return false
	}

	hexChars := "0123456789ABCDEFabcdef"
	for i, c := range s {
		if i == 0 || i == 9 || i == 14 || i == 19 || i == 24 || i == 37 {
			continue
		}
		if !strings.ContainsRune(hexChars, c) {
			return false
		}
	}

	return true
}

// isHexChar checks if a byte is a hexadecimal character
func isHexChar(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')
}

// isAllHex checks if a string contains only hex characters
func isAllHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractMsiPropertyValue extracts a property value from MSI raw data
// MSI stores properties as contiguous strings like: "ProductCode{GUID}ProductVersion1.0.0"
func extractMsiPropertyValue(data []byte, propertyName string) string {
	// First try ASCII search (most MSI data is ASCII)
	asciiResult := extractPropertyValueASCII(data, propertyName)
	if asciiResult != "" {
		return asciiResult
	}

	// Fallback to UTF-16LE search
	return extractPropertyValueUTF16(data, propertyName)
}

// extractPropertyValueASCII searches for property value in ASCII format
func extractPropertyValueASCII(data []byte, propertyName string) string {
	propBytes := []byte(propertyName)
	propLen := len(propBytes)

	for i := 0; i <= len(data)-propLen; i++ {
		if bytes.Equal(data[i:i+propLen], propBytes) {
			// Found property name, extract value immediately after
			valueStart := i + propLen

			// Determine value type based on property name
			switch propertyName {
			case "ProductCode", "UpgradeCode":
				return extractGUIDAfterPosition(data, valueStart)
			case "ProductVersion":
				result := extractVersionAfterPositionStrict(data, valueStart)
				if result != "" {
					return result
				}
			case "Manufacturer", "ProductName":
				result := extractManufacturerAfterPosition(data, valueStart)
				if result != "" {
					return result
				}
			default:
				result := extractStringAfterPosition(data, valueStart)
				if result != "" {
					return result
				}
			}
		}
	}
	return ""
}

// extractPropertyValueUTF16 searches for property value in UTF-16LE format
func extractPropertyValueUTF16(data []byte, propertyName string) string {
	propBytes := stringToUTF16LE(propertyName)
	propLen := len(propBytes)

	for i := 0; i <= len(data)-propLen; i++ {
		if bytes.Equal(data[i:i+propLen], propBytes) {
			valueStart := i + propLen

			switch propertyName {
			case "ProductCode", "UpgradeCode":
				return extractGUIDAfterPositionUTF16(data, valueStart)
			case "ProductVersion":
				return extractVersionAfterPositionUTF16(data, valueStart)
			case "Manufacturer":
				return extractStringAfterPositionUTF16(data, valueStart)
			default:
				return extractStringAfterPositionUTF16(data, valueStart)
			}
		}
	}
	return ""
}

// extractGUIDAfterPosition extracts a GUID immediately after the given position
func extractGUIDAfterPosition(data []byte, pos int) string {
	// Look for '{' within a short range (skip any null bytes or padding)
	searchEnd := min(pos+64, len(data))
	for i := pos; i < searchEnd; i++ {
		if data[i] == '{' {
			guid := extractGUIDAt(data, i)
			if guid != "" {
				return guid
			}
		}
	}
	return ""
}

// extractGUIDAfterPositionUTF16 extracts a GUID in UTF-16LE format
func extractGUIDAfterPositionUTF16(data []byte, pos int) string {
	searchEnd := min(pos+128, len(data))
	for i := pos; i < searchEnd-1; i += 2 {
		if i+1 < len(data) {
			char := binary.LittleEndian.Uint16(data[i:])
			if char == '{' {
				// Extract UTF-16LE GUID
				guid := extractUTF16GUID(data[i:])
				if isValidGUID(guid) {
					return guid
				}
			}
		}
	}
	return ""
}

// extractUTF16GUID extracts a GUID from UTF-16LE data
func extractUTF16GUID(data []byte) string {
	var chars []rune
	for i := 0; i+1 < len(data) && len(chars) < 40; i += 2 {
		char := binary.LittleEndian.Uint16(data[i:])
		if char == 0 {
			break
		}
		chars = append(chars, rune(char))
		if len(chars) == 38 {
			break
		}
	}
	return string(chars)
}

// extractVersionAfterPosition extracts a version string immediately after the given position
func extractVersionAfterPosition(data []byte, pos int) string {
	// Skip any non-digit characters (nulls, padding)
	searchEnd := min(pos+32, len(data))
	for i := pos; i < searchEnd; i++ {
		if data[i] >= '0' && data[i] <= '9' {
			// Found start of version, extract it
			var version []byte
			for j := i; j < min(i+32, len(data)); j++ {
				c := data[j]
				if (c >= '0' && c <= '9') || c == '.' {
					version = append(version, c)
				} else {
					break
				}
			}
			v := string(version)
			if isValidVersion(v) {
				return v
			}
		}
	}
	return ""
}

// extractVersionAfterPositionStrict extracts version starting immediately at position
// This handles MSI's concatenated property format: "ProductVersion8.8.8UpgradeCode..."
func extractVersionAfterPositionStrict(data []byte, pos int) string {
	if pos >= len(data) {
		return ""
	}

	// Version should start immediately with a digit
	if data[pos] < '0' || data[pos] > '9' {
		return ""
	}

	var version []byte
	for i := pos; i < min(pos+32, len(data)); i++ {
		c := data[i]
		if (c >= '0' && c <= '9') || c == '.' {
			version = append(version, c)
		} else {
			// Stop at any non-version character (including letters like 'U' in UpgradeCode)
			break
		}
	}

	v := string(version)
	// Trim trailing dots
	v = strings.TrimSuffix(v, ".")
	if isValidVersion(v) {
		return v
	}
	return ""
}

// extractManufacturerAfterPosition extracts manufacturer string from concatenated MSI data
// Format: "ManufacturerSome Company NameProductCode{...}"
func extractManufacturerAfterPosition(data []byte, pos int) string {
	if pos >= len(data) {
		return ""
	}

	// Look for next known property name to find the end boundary
	knownProperties := []string{"ProductCode", "ProductLanguage", "ProductName", "ProductVersion", "UpgradeCode", "SecureCustomProperties"}

	var result []byte
	maxLen := min(pos+256, len(data))

	for i := pos; i < maxLen; i++ {
		c := data[i]

		// Check if we've hit a known property name
		hitProperty := false
		for _, prop := range knownProperties {
			if i+len(prop) <= len(data) && string(data[i:i+len(prop)]) == prop {
				hitProperty = true
				break
			}
		}
		if hitProperty {
			break
		}

		// Accept printable ASCII characters
		if c >= 32 && c < 127 {
			result = append(result, c)
		} else {
			// Stop at non-printable
			break
		}
	}

	str := strings.TrimSpace(string(result))
	// Validate: reasonable length and looks like a name
	if len(str) >= 2 && len(str) <= 128 {
		if !isValidGUID(str) && !isValidVersion(str) && isValidProductName(str) {
			return str
		}
	}
	return ""
}

// isValidProductName validates that extracted product name is not UI dialog text
func isValidProductName(s string) bool {
	// Reject if too long or too short
	if len(s) < 2 || len(s) > 100 {
		return false
	}

	// Reject if starts/ends with brackets or other suspicious punctuation
	if strings.HasPrefix(s, "]") || strings.HasPrefix(s, "[") ||
		strings.HasSuffix(s, "[") || strings.HasSuffix(s, "]") ||
		strings.HasPrefix(s, ")") || strings.HasPrefix(s, "(") {
		return false
	}

	// Reject UI dialog text patterns commonly found in MSI installers
	lower := strings.ToLower(s)
	invalidPatterns := []string{
		"setup wizard",
		"allows you",
		"the way",
		"change the",
		"is installed",
		"click next",
		"click back",
		"to continue",
		"will be installed",
		"installation",
		"completing the",
		"welcome to",
		"please wait",
	}
	for _, pattern := range invalidPatterns {
		if strings.Contains(lower, pattern) {
			return false
		}
	}

	return true
}

// extractVersionAfterPositionUTF16 extracts a version string in UTF-16LE format
func extractVersionAfterPositionUTF16(data []byte, pos int) string {
	searchEnd := min(pos+64, len(data))
	for i := pos; i < searchEnd-1; i += 2 {
		if i+1 < len(data) {
			char := binary.LittleEndian.Uint16(data[i:])
			if char >= '0' && char <= '9' {
				version := extractUTF16Version(data[i:])
				if isValidVersion(version) {
					return version
				}
			}
		}
	}
	return ""
}

// extractStringAfterPosition extracts a string value immediately after the given position
func extractStringAfterPosition(data []byte, pos int) string {
	// Skip any null bytes or non-printable characters
	searchEnd := min(pos+256, len(data))
	for i := pos; i < searchEnd; i++ {
		c := data[i]
		// Look for start of printable string (uppercase letter or digit)
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			// Extract the string
			var str []byte
			for j := i; j < min(i+256, len(data)); j++ {
				ch := data[j]
				// Allow alphanumeric, space, and common punctuation
				if (ch >= 32 && ch < 127) {
					str = append(str, ch)
				} else {
					break
				}
			}
			result := strings.TrimSpace(string(str))
			// Validate: should be reasonable length and not look like code/paths
			if len(result) >= 2 && len(result) <= 128 {
				// Skip if it looks like a GUID or version
				if !isValidGUID(result) && !isValidVersion(result) {
					// Skip if it's just numbers or single word with no meaning
					if strings.Contains(result, " ") || len(result) >= 3 {
						return result
					}
				}
			}
		}
	}
	return ""
}

// extractStringAfterPositionUTF16 extracts a string value in UTF-16LE format
func extractStringAfterPositionUTF16(data []byte, pos int) string {
	searchEnd := min(pos+512, len(data))
	for i := pos; i < searchEnd-1; i += 2 {
		if i+1 < len(data) {
			char := binary.LittleEndian.Uint16(data[i:])
			// Look for start of printable string
			if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') {
				str := extractUTF16StringClean(data[i:])
				if len(str) >= 2 && len(str) <= 128 {
					if !isValidGUID(str) && !isValidVersion(str) {
						return str
					}
				}
			}
		}
	}
	return ""
}
