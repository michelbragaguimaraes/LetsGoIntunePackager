package packager

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/richardlehane/mscfb"
	"github.com/richardlehane/msoleps"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// MsiInfo contains metadata extracted from an MSI file.
//
// The first group is read verbatim from the MSI Property table and the Summary
// Information stream. The second group is derived from the package contents and
// feeds the MsiInfo block of Detection.xml.
type MsiInfo struct {
	ProductCode    string // {GUID} from Property table
	ProductVersion string // Version from Property table
	PackageCode    string // {GUID} from Summary Information
	Publisher      string // Manufacturer from Property table
	UpgradeCode    string // {GUID} from Property table
	ProductName    string // ProductName from Property table (for display)

	// ExecutionContext is "System", "User" or "Any", derived from ALLUSERS.
	ExecutionContext string
	// IsMachineInstall and IsUserInstall are the ALLUSERS setting expressed as
	// the pair of booleans Detection.xml carries.
	IsMachineInstall bool
	IsUserInstall    bool
	// IncludesServices is true when the package has a non-empty ServiceInstall
	// table, IncludesODBCDataSource likewise for ODBCDataSource.
	IncludesServices       bool
	IncludesODBCDataSource bool
}

// IsMsiFile checks if the given file path has an .msi extension
func IsMsiFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".msi")
}

// msiBase64 is the alphabet used by the MSI stream-name encoding. Note that it
// is not RFC 4648 base64: the digits come first.
const msiBase64 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz._"

// tableNamePrefix marks a stream that holds a database table. MSI prepends it
// before encoding, and it is left as-is by the encoding.
const tableNamePrefix = 0x4840

// decodeStreamName reverses the mangling MSI applies to stream names. Names are
// packed into the private-use range U+3800..U+4840, two characters per rune for
// U+3800..U+4800 and one per rune above that, so a table such as Property is
// stored under a name that bears no textual resemblance to "Property".
//
// The second return value reports whether the stream holds a database table.
func decodeStreamName(name string) (string, bool) {
	runes := []rune(name)
	isTable := false
	if len(runes) > 0 && runes[0] == tableNamePrefix {
		isTable = true
		runes = runes[1:]
	}

	var sb strings.Builder
	for _, ch := range runes {
		switch {
		case ch >= 0x3800 && ch < 0x4800:
			v := int(ch - 0x3800)
			sb.WriteByte(msiBase64[v&0x3f])
			sb.WriteByte(msiBase64[(v>>6)&0x3f])
		case ch >= 0x4800 && ch < tableNamePrefix:
			sb.WriteByte(msiBase64[int(ch-0x4800)&0x3f])
		default:
			sb.WriteRune(ch)
		}
	}
	return sb.String(), isTable
}

// msiCodepages maps the Windows codepage identifiers that appear in MSI string
// pools onto decoders. Codepages 0 and 65001 are deliberately absent: both mean
// the pool is already UTF-8 and needs no conversion.
var msiCodepages = map[int]encoding.Encoding{
	437:   charmap.CodePage437,
	850:   charmap.CodePage850,
	852:   charmap.CodePage852,
	855:   charmap.CodePage855,
	858:   charmap.CodePage858,
	860:   charmap.CodePage860,
	862:   charmap.CodePage862,
	866:   charmap.CodePage866,
	874:   charmap.Windows874,
	932:   japanese.ShiftJIS,
	936:   simplifiedchinese.GBK,
	949:   korean.EUCKR,
	950:   traditionalchinese.Big5,
	1250:  charmap.Windows1250,
	1251:  charmap.Windows1251,
	1252:  charmap.Windows1252,
	1253:  charmap.Windows1253,
	1254:  charmap.Windows1254,
	1255:  charmap.Windows1255,
	1256:  charmap.Windows1256,
	1257:  charmap.Windows1257,
	1258:  charmap.Windows1258,
	20866: charmap.KOI8R,
	21866: charmap.KOI8U,
	28591: charmap.ISO8859_1,
	28592: charmap.ISO8859_2,
	28593: charmap.ISO8859_3,
	28594: charmap.ISO8859_4,
	28595: charmap.ISO8859_5,
	28596: charmap.ISO8859_6,
	28597: charmap.ISO8859_7,
	28598: charmap.ISO8859_8,
	28599: charmap.ISO8859_9,
	28603: charmap.ISO8859_13,
	28605: charmap.ISO8859_15,
	50220: japanese.ISO2022JP,
	51932: japanese.EUCJP,
	54936: simplifiedchinese.GB18030,
}

// decodeMSIString converts a raw string-pool entry to UTF-8 using the codepage
// the pool declares. Decoding a Shift-JIS pool as Windows-1252 turns a Japanese
// publisher name into mojibake, so the declaration is honoured rather than
// guessed at.
func decodeMSIString(b []byte, codepage int) string {
	if codepage == 0 || codepage == 65001 {
		return string(b)
	}

	enc, known := msiCodepages[codepage]
	if !known {
		// An unrecognised codepage is worth one guess: UTF-8 when the bytes
		// parse as it, otherwise Windows-1252, much the commonest single-byte
		// pool encoding.
		if utf8.Valid(b) {
			return string(b)
		}
		enc = charmap.Windows1252
	}

	out, _, err := transform.Bytes(enc.NewDecoder(), b)
	if err != nil {
		return string(b)
	}
	return string(out)
}

// parseStringPool decodes the MSI string pool. _StringPool is an array of
// 4-byte entries (a 16-bit length and a 16-bit reference count) describing how
// to slice _StringData, which is one unseparated run of bytes.
//
// It returns the strings indexed by string ID -- IDs are 1-based, so index 0 is
// always empty -- and the width in bytes of a string reference elsewhere in the
// database.
func parseStringPool(pool, data []byte) (strs []string, bytesPerRef int) {
	bytesPerRef = 2
	if len(pool) < 4 {
		return nil, bytesPerRef
	}

	entries := len(pool) / 4
	u16 := func(i int) int {
		if (i+1)*2 > len(pool) {
			return 0
		}
		return int(binary.LittleEndian.Uint16(pool[i*2:]))
	}

	// Entry 0 is a header rather than a string: it carries the codepage, and
	// the high bit of its second word widens every string reference to 3 bytes.
	header := u16(1)
	if header&0x8000 != 0 {
		bytesPerRef = 3
	}
	codepage := u16(0) | ((header &^ 0x8000) << 16)

	strs = make([]string, 1, entries)
	offset := 0
	for i := 1; i < entries; {
		length, refs := u16(i*2), u16(i*2+1)

		// A wholly zero entry is an unused slot that still consumes an ID.
		if length == 0 && refs == 0 {
			strs = append(strs, "")
			i++
			continue
		}

		if length == 0 {
			// Strings over 64k null out their own entry and store the real
			// length across the following one.
			if i+1 >= entries {
				break
			}
			length = u16(i*2+2) | (u16(i*2+3) << 16)
			i += 2
		} else {
			i++
		}

		if offset+length > len(data) {
			break
		}
		strs = append(strs, decodeMSIString(data[offset:offset+length], codepage))
		offset += length
	}
	return strs, bytesPerRef
}

// readStringTable reads a table stream whose columns are all string references.
// MSI stores tables column-major: every value of column 0 for all rows, then
// every value of column 1, and so on.
func readStringTable(stream []byte, numCols, bytesPerRef int, pool []string) [][]string {
	if numCols <= 0 || bytesPerRef <= 0 {
		return nil
	}
	rowWidth := numCols * bytesPerRef
	if len(stream) < rowWidth {
		return nil
	}
	rows := len(stream) / rowWidth

	lookup := func(col, row int) string {
		off := (col*rows + row) * bytesPerRef
		if off+bytesPerRef > len(stream) {
			return ""
		}
		id := int(stream[off]) | int(stream[off+1])<<8
		if bytesPerRef == 3 {
			id |= int(stream[off+2]) << 16
		}
		if id <= 0 || id >= len(pool) {
			return ""
		}
		return pool[id]
	}

	out := make([][]string, rows)
	for r := range out {
		row := make([]string, numCols)
		for c := range row {
			row[c] = lookup(c, r)
		}
		out[r] = row
	}
	return out
}

// ExtractMsiInfo reads metadata from an MSI by parsing its database tables.
//
// The MSI is an OLE compound file whose streams hold the Property table, the
// string pool it refers into, and the Summary Information property set. Values
// are read from those structures rather than scanned for, so failing to find a
// property yields an empty field instead of a neighbouring package's GUID.
func ExtractMsiInfo(msiPath string) (*MsiInfo, error) {
	file, err := os.Open(msiPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open MSI file: %w", err)
	}
	defer file.Close()

	doc, err := mscfb.New(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MSI as OLE document: %w", err)
	}

	// Only the handful of streams below are of interest, but the reader is
	// forward-only so each entry has to be consumed as it is visited.
	wanted := map[string]bool{
		"_StringPool": true, "_StringData": true, "Property": true,
		"ServiceInstall": true, "ODBCDataSource": true,
	}
	tables := make(map[string][]byte, len(wanted))
	var summary []byte

	for entry, nextErr := doc.Next(); nextErr == nil; entry, nextErr = doc.Next() {
		name, isTable := decodeStreamName(entry.Name)

		// Summary Information is neither a table nor name-mangled. Its leading
		// \x05 is stripped by some CFB readers, so accept the name either way.
		if !isTable && strings.TrimPrefix(name, "\x05") == "SummaryInformation" {
			summary, _ = io.ReadAll(entry)
			continue
		}
		if !isTable || !wanted[name] {
			continue
		}
		if data, readErr := io.ReadAll(entry); readErr == nil {
			tables[name] = data
		}
	}

	pool, bytesPerRef := parseStringPool(tables["_StringPool"], tables["_StringData"])
	if len(pool) <= 1 {
		return nil, fmt.Errorf("MSI string pool is missing or unreadable")
	}

	// The Property table schema is fixed by the MSI specification: two string
	// columns, Property and Value.
	props := make(map[string]string)
	for _, row := range readStringTable(tables["Property"], 2, bytesPerRef, pool) {
		if row[0] != "" {
			props[row[0]] = row[1]
		}
	}
	if len(props) == 0 {
		return nil, fmt.Errorf("MSI Property table is missing or empty")
	}

	info := &MsiInfo{
		ProductCode:            props["ProductCode"],
		ProductVersion:         props["ProductVersion"],
		UpgradeCode:            props["UpgradeCode"],
		Publisher:              props["Manufacturer"],
		ProductName:            props["ProductName"],
		PackageCode:            extractPackageCode(summary),
		IncludesServices:       len(tables["ServiceInstall"]) > 0,
		IncludesODBCDataSource: len(tables["ODBCDataSource"]) > 0,
	}
	info.ExecutionContext, info.IsMachineInstall, info.IsUserInstall = executionContext(props["ALLUSERS"])

	return info, nil
}

// executionContext maps the ALLUSERS property onto the install-context fields
// of Detection.xml. ALLUSERS=1 installs per-machine, 2 installs per-machine
// where privileges allow and per-user otherwise, and absent or 0 is per-user.
func executionContext(allUsers string) (context string, machine, user bool) {
	switch strings.TrimSpace(allUsers) {
	case "1":
		return "System", true, false
	case "2":
		return "Any", true, false
	default:
		return "User", false, true
	}
}

// revisionNumberNames are the spellings different readers give to property 9 of
// the Summary Information set, which for an MSI holds the PackageCode.
var revisionNumberNames = map[string]bool{
	"RevNumber":       true,
	"PIDSI_REVNUMBER": true,
	"Revision Number": true,
}

// extractPackageCode reads the PackageCode from the Summary Information stream.
func extractPackageCode(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	props, err := msoleps.NewFrom(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	for _, prop := range props.Property {
		if !revisionNumberNames[prop.Name] {
			continue
		}
		str := fmt.Sprintf("%v", prop)
		if isValidGUID(str) {
			return str
		}
		// Patches and transforms store the code in MSI's packed GUID form.
		if guid := decompressMSIGUID(str); guid != "" {
			return guid
		}
	}
	return ""
}

// decompressMSIGUID converts MSI's compressed GUID format to standard GUID
// format. MSI reverses each group of the GUID, so the 32 hex characters have to
// be reordered rather than merely punctuated.
func decompressMSIGUID(compressed string) string {
	compressed = strings.TrimSpace(compressed)
	if len(compressed) != 32 || !isAllHex(compressed) {
		return ""
	}

	reverse := func(s string) string {
		b := []byte(s)
		for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
			b[i], b[j] = b[j], b[i]
		}
		return string(b)
	}
	// The first three groups are reversed wholesale; the last two are stored as
	// byte pairs with the two nibbles of each pair swapped.
	swapPairs := func(s string) string {
		var sb strings.Builder
		for i := 0; i+1 < len(s); i += 2 {
			sb.WriteByte(s[i+1])
			sb.WriteByte(s[i])
		}
		return sb.String()
	}

	return strings.ToUpper(fmt.Sprintf("{%s-%s-%s-%s-%s}",
		reverse(compressed[0:8]),
		reverse(compressed[8:12]),
		reverse(compressed[12:16]),
		swapPairs(compressed[16:20]),
		swapPairs(compressed[20:32]),
	))
}

// isValidGUID reports whether s is a {XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX} GUID
func isValidGUID(s string) bool {
	if len(s) != 38 || s[0] != '{' || s[37] != '}' {
		return false
	}
	for i, c := range s[1:37] {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !isHexChar(byte(c)) {
				return false
			}
		}
	}
	return true
}

// isHexChar checks if a byte is a hexadecimal character
func isHexChar(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// isAllHex checks if a string contains only hex characters
func isAllHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isHexChar(s[i]) {
			return false
		}
	}
	return true
}
