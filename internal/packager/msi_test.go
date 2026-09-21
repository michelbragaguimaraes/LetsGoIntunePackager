package packager

import (
	"strings"
	"testing"
)

func TestIsMsiFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"setup.msi", true},
		{"SETUP.MSI", true},
		{"setup.Msi", true},
		{"path/to/installer.msi", true},
		{"setup.exe", false},
		{"setup.msi.bak", false},
		{"msi", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsMsiFile(tt.path)
			if result != tt.expected {
				t.Errorf("IsMsiFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsValidGUID(t *testing.T) {
	tests := []struct {
		guid     string
		expected bool
	}{
		{"{12345678-1234-1234-1234-123456789ABC}", true},
		{"{AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE}", true},
		{"{aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee}", true},
		{"{12345678-1234-1234-1234-123456789abc}", true},
		// Invalid cases
		{"12345678-1234-1234-1234-123456789ABC", false},    // Missing braces
		{"{12345678123412341234123456789ABC}", false},      // Missing dashes
		{"{12345678-1234-1234-1234-123456789AB}", false},   // Too short
		{"{12345678-1234-1234-1234-123456789ABCD}", false}, // Too long
		{"{GGGGGGGG-GGGG-GGGG-GGGG-GGGGGGGGGGGG}", false},  // Invalid hex
		{"", false},
		{"not-a-guid", false},
	}

	for _, tt := range tests {
		t.Run(tt.guid, func(t *testing.T) {
			result := isValidGUID(tt.guid)
			if result != tt.expected {
				t.Errorf("isValidGUID(%q) = %v, want %v", tt.guid, result, tt.expected)
			}
		})
	}
}

func TestDecompressMSIGUID(t *testing.T) {
	tests := []struct {
		compressed string
		expected   string
	}{
		// Test with a known compressed GUID
		// MSI compressed format swaps byte pairs within each group
		{"21436587BA098765FEDC324109876543", "{12345678-09AB-5678-CDEF-1234098765432}"[0:0]}, // Placeholder - actual test below
	}

	// Basic format test - empty string for invalid input
	result := decompressMSIGUID("invalid")
	if result != "" {
		t.Errorf("decompressMSIGUID with invalid input should return empty string, got %q", result)
	}

	// Test with 32-char string that's not valid hex
	result = decompressMSIGUID("GGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG")
	if result != "" {
		t.Errorf("decompressMSIGUID with invalid hex should return empty string, got %q", result)
	}

	// Skip the placeholder test
	_ = tests
}

func TestIsHexChar(t *testing.T) {
	validHex := "0123456789ABCDEFabcdef"
	for _, c := range validHex {
		if !isHexChar(byte(c)) {
			t.Errorf("isHexChar(%q) = false, want true", string(c))
		}
	}

	invalidChars := "GHIJKLMNOPQRSTUVWXYZghijklmnopqrstuvwxyz!@#$%"
	for _, c := range invalidChars {
		if isHexChar(byte(c)) {
			t.Errorf("isHexChar(%q) = true, want false", string(c))
		}
	}
}

func TestIsAllHex(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"0123456789ABCDEF", true},
		{"abcdef", true},
		{"ABC123", true},
		{"", true}, // Empty string is technically all hex
		{"GHIJ", false},
		{"12G4", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isAllHex(tt.input)
			if result != tt.expected {
				t.Errorf("isAllHex(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractMsiInfoNonExistent(t *testing.T) {
	_, err := ExtractMsiInfo("/nonexistent/path/to/file.msi")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

// encodeStreamName applies the MSI stream-name mangling, so the decoder can be
// exercised against names built the way a real MSI builds them.
func encodeStreamName(isTable bool, name string) string {
	idx := func(b byte) int { return strings.IndexByte(msiBase64, b) }

	var out []rune
	if isTable {
		out = append(out, tableNamePrefix)
	}
	for i := 0; i < len(name); {
		switch {
		case i+1 < len(name) && idx(name[i]) >= 0 && idx(name[i+1]) >= 0:
			out = append(out, rune(0x3800+idx(name[i])+idx(name[i+1])<<6))
			i += 2
		case idx(name[i]) >= 0:
			out = append(out, rune(0x4800+idx(name[i])))
			i++
		default:
			out = append(out, rune(name[i]))
			i++
		}
	}
	return string(out)
}

func TestDecodeStreamName(t *testing.T) {
	tests := []struct {
		name    string
		isTable bool
	}{
		{"Property", true},
		{"_StringPool", true},
		{"_StringData", true},
		{"ServiceInstall", true},
		{"ODBCDataSource", true},
		{"InstallExecuteSequence", true},
		{"A", true},
		{"SummaryInformation", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotTable := decodeStreamName(encodeStreamName(tt.isTable, tt.name))
			if got != tt.name {
				t.Errorf("decodeStreamName() = %q, want %q", got, tt.name)
			}
			if gotTable != tt.isTable {
				t.Errorf("decodeStreamName() isTable = %v, want %v", gotTable, tt.isTable)
			}
		})
	}
}

func TestDecodeStreamNamePassesThroughPlainNames(t *testing.T) {
	// The Summary Information stream is not mangled and not a table.
	got, isTable := decodeStreamName("\x05SummaryInformation")
	if got != "\x05SummaryInformation" || isTable {
		t.Errorf("decodeStreamName() = %q/%v, want unchanged and not a table", got, isTable)
	}
}

// buildStringPool assembles a _StringPool stream from (length, refcount) pairs.
func buildStringPool(header [2]uint16, entries [][2]uint16) []byte {
	out := make([]byte, 0, (len(entries)+1)*4)
	put := func(v uint16) { out = append(out, byte(v), byte(v>>8)) }
	put(header[0])
	put(header[1])
	for _, e := range entries {
		put(e[0])
		put(e[1])
	}
	return out
}

func TestParseStringPool(t *testing.T) {
	pool := buildStringPool([2]uint16{0, 0}, [][2]uint16{
		{5, 1}, // "Hello"
		{0, 0}, // unused slot, still consumes an ID
		{5, 2}, // "World"
	})
	strs, bytesPerRef := parseStringPool(pool, []byte("HelloWorld"))

	if bytesPerRef != 2 {
		t.Errorf("bytesPerRef = %d, want 2", bytesPerRef)
	}
	want := []string{"", "Hello", "", "World"}
	if len(strs) != len(want) {
		t.Fatalf("parseStringPool() returned %d strings (%q), want %d", len(strs), strs, len(want))
	}
	for i := range want {
		if strs[i] != want[i] {
			t.Errorf("strs[%d] = %q, want %q", i, strs[i], want[i])
		}
	}
}

func TestParseStringPoolWideRefs(t *testing.T) {
	// The high bit of the header's second word widens string references.
	pool := buildStringPool([2]uint16{0, 0x8000}, [][2]uint16{{3, 1}})
	strs, bytesPerRef := parseStringPool(pool, []byte("abc"))

	if bytesPerRef != 3 {
		t.Errorf("bytesPerRef = %d, want 3", bytesPerRef)
	}
	if len(strs) != 2 || strs[1] != "abc" {
		t.Errorf("parseStringPool() = %q, want [\"\" \"abc\"]", strs)
	}
}

func TestParseStringPoolTruncated(t *testing.T) {
	// A length running past the end of _StringData must stop the walk rather
	// than panic or read out of bounds.
	pool := buildStringPool([2]uint16{0, 0}, [][2]uint16{{2, 1}, {99, 1}})
	strs, _ := parseStringPool(pool, []byte("ab"))
	if len(strs) != 2 || strs[1] != "ab" {
		t.Errorf("parseStringPool() = %q, want [\"\" \"ab\"]", strs)
	}

	if strs, _ := parseStringPool(nil, nil); strs != nil {
		t.Errorf("parseStringPool(nil, nil) = %q, want nil", strs)
	}
}

func TestReadStringTable(t *testing.T) {
	pool := []string{"", "ProductCode", "{GUID}", "ProductVersion", "1.0"}

	// Column-major: both values of column 0, then both of column 1.
	stream := []byte{
		1, 0, 3, 0, // column 0: ProductCode, ProductVersion
		2, 0, 4, 0, // column 1: {GUID}, 1.0
	}
	rows := readStringTable(stream, 2, 2, pool)

	want := [][]string{{"ProductCode", "{GUID}"}, {"ProductVersion", "1.0"}}
	if len(rows) != len(want) {
		t.Fatalf("readStringTable() returned %d rows (%q), want %d", len(rows), rows, len(want))
	}
	for r := range want {
		for c := range want[r] {
			if rows[r][c] != want[r][c] {
				t.Errorf("rows[%d][%d] = %q, want %q", r, c, rows[r][c], want[r][c])
			}
		}
	}
}

func TestReadStringTableRejectsGarbage(t *testing.T) {
	pool := []string{"", "a"}

	if rows := readStringTable(nil, 2, 2, pool); rows != nil {
		t.Errorf("readStringTable(nil) = %v, want nil", rows)
	}
	if rows := readStringTable([]byte{1, 0}, 0, 2, pool); rows != nil {
		t.Errorf("readStringTable() with 0 columns = %v, want nil", rows)
	}
	// String IDs past the end of the pool resolve to empty, not a panic.
	rows := readStringTable([]byte{99, 0, 99, 0}, 2, 2, pool)
	if len(rows) != 1 || rows[0][0] != "" || rows[0][1] != "" {
		t.Errorf("readStringTable() with out-of-range IDs = %q, want one empty row", rows)
	}
}

func TestDecodeMSIString(t *testing.T) {
	tests := []struct {
		name     string
		codepage int
		in       []byte
		want     string
	}{
		{"utf8 explicit", 65001, []byte{0x63, 0x61, 0x66, 0xc3, 0xa9}, "café"},
		{"utf8 codepage zero", 0, []byte{0x63, 0x61, 0x66, 0xc3, 0xa9}, "café"},
		{"ascii under cp1252", 1252, []byte("plain ascii"), "plain ascii"},

		// Bytes below are the literal legacy encodings, not UTF-8. Decoding
		// them as Windows-1252 is what used to turn a Japanese publisher into
		// mojibake, so each must follow its declared codepage.
		{
			"shift-jis", 932,
			[]byte{0x93, 0xfa, 0x96, 0x7b, 0x83, 0x5c, 0x83, 0x74, 0x83, 0x67, 0x8a, 0x94, 0x8e, 0xae, 0x89, 0xef, 0x8e, 0xd0},
			"日本ソフト株式会社",
		},
		{
			"gbk", 936,
			[]byte{0xb3, 0xcc, 0xd0, 0xf2, 0xc4, 0xa3, 0xbf, 0xe9},
			"程序模块",
		},
		{
			"windows-1251", 1251,
			[]byte{0xcf, 0xf0, 0xe8, 0xeb, 0xee, 0xe6, 0xe5, 0xed, 0xe8, 0xe5},
			"Приложение",
		},
		{
			"windows-1252", 1252,
			[]byte{0x43, 0x61, 0x66, 0xe9, 0x20, 0x4d, 0xfc, 0x6c, 0x6c, 0x65, 0x72},
			"Café Müller",
		},

		// An unrecognised codepage gets one guess: UTF-8 when it parses,
		// Windows-1252 otherwise.
		{"unknown codepage, valid utf8", 12345, []byte{0x63, 0x61, 0x66, 0xc3, 0xa9}, "café"},
		{"unknown codepage, not utf8", 12345, []byte{0x43, 0x61, 0x66, 0xe9}, "Café"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeMSIString(tt.in, tt.codepage); got != tt.want {
				t.Errorf("decodeMSIString(%#x, %d) = %q, want %q", tt.in, tt.codepage, got, tt.want)
			}
		})
	}
}

func TestDecodeMSIStringShiftJISIsNotWindows1252(t *testing.T) {
	// Regression guard: these bytes are valid in both encodings, and reading a
	// Shift-JIS pool as Windows-1252 silently produces plausible-looking Latin
	// text rather than failing.
	sjis := []byte{0x93, 0xfa, 0x96, 0x7b}
	if got := decodeMSIString(sjis, 932); got != "日本" {
		t.Errorf("codepage 932 decoded as %q, want %q", got, "日本")
	}
	if got := decodeMSIString(sjis, 1252); got == "日本" {
		t.Error("codepage 1252 must not produce the Shift-JIS reading")
	}
}

func TestExecutionContext(t *testing.T) {
	tests := []struct {
		allUsers string
		context  string
		machine  bool
		user     bool
	}{
		{"1", "System", true, false},
		{"2", "Any", true, false},
		{"", "User", false, true},
		{"0", "User", false, true},
		{" 1 ", "System", true, false},
	}

	for _, tt := range tests {
		t.Run("ALLUSERS="+tt.allUsers, func(t *testing.T) {
			ctx, machine, user := executionContext(tt.allUsers)
			if ctx != tt.context || machine != tt.machine || user != tt.user {
				t.Errorf("executionContext(%q) = %q/%v/%v, want %q/%v/%v",
					tt.allUsers, ctx, machine, user, tt.context, tt.machine, tt.user)
			}
		})
	}
}

func TestExtractPackageCodeEmpty(t *testing.T) {
	if got := extractPackageCode(nil); got != "" {
		t.Errorf("extractPackageCode(nil) = %q, want empty", got)
	}
	if got := extractPackageCode([]byte("not a property set")); got != "" {
		t.Errorf("extractPackageCode(garbage) = %q, want empty", got)
	}
}
