package main

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dsoprea/go-exif/v3"
)

func main() {
	// CLI flags
	jsonOut := flag.Bool("json", false, "出力をJSONにします")       // --json
	rawOut := flag.Bool("raw", false, "デバッグ用に生のメタデータを表示します") // --raw
	pretty := flag.Bool("pretty", false, "JSONを整形して出力します ( --json と併用 )")
	noEscape := flag.Bool("no-escape", false, "JSON出力時にHTMLエスケープを無効化します（危険）")
	ndjson := flag.Bool("ndjson", false, "JSON出力を1行ごとのNDJSONで出力します（--json と併用）")
	verbose := flag.Bool("verbose", false, "詳細な人間向け出力を有効化します（--json時はstderrに出力）")
	noHuman := flag.Bool("no-human", false, "人間向け出力を全て抑制します（--jsonと併用して純粋なJSONのみ出力する）")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("画像ファイルをドラッグ＆ドロップしてください。")
		return
	}

	// If JSON output is requested, collect or stream JSON-only output
	if *jsonOut {
		if *ndjson {
			// Stream NDJSON: one JSON object per file, newline-delimited
			for _, path := range flag.Args() {
				meta, err := readVRChatExifPNG(path, *jsonOut, *rawOut, *pretty, *noEscape, *verbose, *noHuman)
				if err != nil {
					fmt.Fprintf(os.Stderr, "エラー (%s): %v\n", path, err)
					continue
				}
				// Use encoder to control escaping
				enc := json.NewEncoder(os.Stdout)
				if *noEscape {
					enc.SetEscapeHTML(false)
				}
				// NDJSON typically shouldn't be pretty-printed
				if err := enc.Encode(meta); err != nil {
					fmt.Fprintf(os.Stderr, "JSON書き出しエラー (%s): %v\n", path, err)
				}
			}
			return
		}

		// Collect all metas into a JSON array
		var all []map[string]interface{}
		for _, path := range flag.Args() {
			meta, err := readVRChatExifPNG(path, *jsonOut, *rawOut, *pretty, *noEscape, *verbose, *noHuman)
			if err != nil {
				fmt.Fprintf(os.Stderr, "エラー (%s): %v\n", path, err)
				continue
			}
			all = append(all, meta)
		}

		// Output array with selected escaping/format
		enc := json.NewEncoder(os.Stdout)
		if *noEscape {
			enc.SetEscapeHTML(false)
		}
		if *pretty {
			enc.SetIndent("", "  ")
		}
		if err := enc.Encode(all); err != nil {
			fmt.Fprintf(os.Stderr, "JSON書き出しエラー: %v\n", err)
		}
		return
	}

	// Non-JSON mode: print human-readable output per file
	for _, path := range flag.Args() {
		fmt.Printf("\n--- ファイル: %s ---\n", path)
		_, _ = readVRChatExifPNG(path, *jsonOut, *rawOut, *pretty, *noEscape, *verbose, *noHuman)
	}

	if !*jsonOut && !*rawOut {
		fmt.Println("\nEnterキーで終了します...")
		var s string
		fmt.Scanln(&s)
	}
}

func extractExifFromPNG(data []byte) ([]byte, error) {
	// PNG files start with an 8-byte signature. They are composed of chunks:
	// [4 bytes length][4 bytes type][length bytes data][4 bytes CRC]
	if len(data) < 8 {
		return nil, errors.New("not a valid PNG")
	}

	offset := 8 // skip PNG signature
	for offset+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		chunkType := string(data[offset+4 : offset+8])
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + length
		chunkCRCEnd := chunkDataEnd + 4

		if chunkDataEnd > len(data) || chunkCRCEnd > len(data) {
			break
		}

		if chunkType == "eXIf" {
			return data[chunkDataStart:chunkDataEnd], nil
		}

		offset = chunkCRCEnd
	}

	return nil, errors.New("eXIf chunk not found")
}

func extractTextualMetadataFromPNG(data []byte) (string, error) {
	if len(data) < 8 {
		return "", errors.New("not a valid PNG")
	}
	offset := 8
	for offset+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		chunkType := string(data[offset+4 : offset+8])
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + length
		chunkCRCEnd := chunkDataEnd + 4

		if chunkDataEnd > len(data) || chunkCRCEnd > len(data) {
			break
		}

		switch chunkType {
		case "tEXt":
			d := data[chunkDataStart:chunkDataEnd]
			if i := bytes.IndexByte(d, 0); i != -1 {
				return string(d[i+1:]), nil
			}
			return string(d), nil
		case "iTXt":
			d := data[chunkDataStart:chunkDataEnd]
			parts := bytes.SplitN(d, []byte{0}, 6)
			if len(parts) >= 6 {
				return string(parts[5]), nil
			} else if len(parts) > 0 {
				return string(parts[len(parts)-1]), nil
			}
		case "zTXt":
			d := data[chunkDataStart:chunkDataEnd]
			if idx := bytes.IndexByte(d, 0); idx != -1 && idx+2 < len(d) {
				compMethod := d[idx+1]
				if compMethod == 0 {
					compData := d[idx+2:]
					r, err := zlib.NewReader(bytes.NewReader(compData))
					if err == nil {
						b, err2 := io.ReadAll(r)
						r.Close()
						if err2 == nil {
							return string(b), nil
						}
					}
				}
			}
		}

		offset = chunkCRCEnd
	}

	return "", errors.New("textual metadata not found")
}

func extractExifFromWebP(data []byte) ([]byte, error) {
	if len(data) < 12 {
		return nil, errors.New("not a valid WebP")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, errors.New("not a valid WebP")
	}
	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + size
		if chunkDataEnd > len(data) {
			break
		}
		if chunkID == "EXIF" {
			return data[chunkDataStart:chunkDataEnd], nil
		}
		offset = chunkDataEnd
		if size%2 == 1 {
			offset++
		}
	}
	return nil, errors.New("EXIF chunk not found")
}

func extractTextualMetadataFromWebP(data []byte) (string, error) {
	if len(data) < 12 {
		return "", errors.New("not a valid WebP")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return "", errors.New("not a valid WebP")
	}
	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + size
		if chunkDataEnd > len(data) {
			break
		}
		if chunkID == "XMP " {
			return string(data[chunkDataStart:chunkDataEnd]), nil
		}
		offset = chunkDataEnd
		if size%2 == 1 {
			offset++
		}
	}
	return "", errors.New("XMP chunk not found")
}

func extractVRChatFromXMP(xmp string) (bool, string, string, string) {
	// Returns found, worldID, worldDisplayName, authorID
	dec := xml.NewDecoder(strings.NewReader(xmp))
	const vrcNS = "http://ns.vrchat.com/vrc/1.0/"
	var worldID, worldName, authorID string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Space == vrcNS {
				switch se.Name.Local {
				case "WorldID":
					var v string
					_ = dec.DecodeElement(&v, &se)
					worldID = strings.TrimSpace(v)
				case "WorldDisplayName":
					var v string
					_ = dec.DecodeElement(&v, &se)
					worldName = strings.TrimSpace(v)
				case "AuthorID":
					var v string
					_ = dec.DecodeElement(&v, &se)
					authorID = strings.TrimSpace(v)
				}
			}
		}
	}
	found := worldID != "" || worldName != "" || authorID != ""
	return found, worldID, worldName, authorID
}

// detectFileType returns a simple file type name
func detectFileType(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return "PNG"
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "WebP"
	}
	if len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8 {
		return "JPEG"
	}
	return "Unknown"
}

func extractPNGDimensions(data []byte) (int, int, error) {
	if len(data) < 24 {
		return 0, 0, errors.New("not a valid PNG for dimension")
	}
	offset := 8
	for offset+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		chunkType := string(data[offset+4 : offset+8])
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + length
		chunkCRCEnd := chunkDataEnd + 4
		if chunkDataEnd > len(data) || chunkCRCEnd > len(data) {
			break
		}
		if chunkType == "IHDR" && length >= 8 {
			width := int(binary.BigEndian.Uint32(data[chunkDataStart : chunkDataStart+4]))
			height := int(binary.BigEndian.Uint32(data[chunkDataStart+4 : chunkDataStart+8]))
			return width, height, nil
		}
		offset = chunkCRCEnd
	}
	return 0, 0, errors.New("IHDR not found")
}

func parseLittle24(b []byte) int {
	return int(b[0]) | int(b[1])<<8 | int(b[2])<<16
}

func extractWebPDimensionsAndFlags(data []byte) (int, int, bool, bool, error) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0, false, false, errors.New("not a valid WebP")
	}
	offset := 12
	var hasAlpha, hasAnim bool
	var width, height int
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkDataStart := offset + 8
		chunkDataEnd := chunkDataStart + size
		if chunkDataEnd > len(data) {
			break
		}
		switch chunkID {
		case "VP8X":
			if size >= 10 {
				b := data[chunkDataStart:chunkDataEnd]
				flags := b[0]
				hasAlpha = (flags & 0x10) != 0
				hasAnim = (flags & 0x02) != 0
				w := parseLittle24(b[4:7])
				h := parseLittle24(b[7:10])
				width = w + 1
				height = h + 1
			}
		case "ALPH":
			hasAlpha = true
		case "ANIM":
			hasAnim = true
		case "VP8 ":
			// Lossy bitstream: try to parse width/height from start of frame (not guaranteed)
			if size >= 10 {
				b := data[chunkDataStart:chunkDataEnd]
				// Frame header at offset 6, 2 bytes little-endian width
				if len(b) >= 10 {
					// Try to read 16-bit little-endian values (may fail for some files)
					w := int(binary.LittleEndian.Uint16(b[6:8]))
					h := int(binary.LittleEndian.Uint16(b[8:10]))
					if w != 0 && h != 0 {
						width = w
						height = h
					}
				}
			}
		case "VP8L":
			// Lossless: width and height are stored in 4 bytes little-endian with 14 and 14 bits
			if size >= 5 {
				b := data[chunkDataStart:chunkDataEnd]
				if len(b) >= 5 {
					packed := uint32(b[1]) | uint32(b[2])<<8 | uint32(b[3])<<16 | uint32(b[4])<<24
					w := int((packed & 0x3FFF) + 1)
					h := int(((packed >> 14) & 0x3FFF) + 1)
					if w != 0 && h != 0 {
						width = w
						height = h
					}
				}
			}
		}
		offset = chunkDataEnd
		if size%2 == 1 {
			offset++
		}
	}
	if width == 0 || height == 0 {
		return width, height, hasAlpha, hasAnim, errors.New("dimensions not found")
	}
	return width, height, hasAlpha, hasAnim, nil
}

func extractAllXMPFields(xmp string) []struct {
	NS    string
	Name  string
	Value string
	Attrs []xml.Attr
} {
	dec := xml.NewDecoder(strings.NewReader(xmp))
	type field struct {
		NS    string
		Name  string
		Value string
		Attrs []xml.Attr
	}
	fieldsMap := map[string]*field{}
	order := []string{}
	stack := []string{}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			key := t.Name.Space + "|" + t.Name.Local
			if _, exists := fieldsMap[key]; !exists {
				fieldsMap[key] = &field{NS: t.Name.Space, Name: t.Name.Local, Attrs: t.Attr}
				order = append(order, key)
			}
			stack = append(stack, key)
		case xml.CharData:
			if len(stack) > 0 {
				k := stack[len(stack)-1]
				v := strings.TrimSpace(string(t))
				if v != "" {
					if fieldsMap[k].Value == "" {
						fieldsMap[k].Value = v
					} else {
						fieldsMap[k].Value += " " + v
					}
				}
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	res := []struct {
		NS    string
		Name  string
		Value string
		Attrs []xml.Attr
	}{}
	for _, k := range order {
		f := fieldsMap[k]
		res = append(res, struct {
			NS    string
			Name  string
			Value string
			Attrs []xml.Attr
		}{NS: f.NS, Name: f.Name, Value: f.Value, Attrs: f.Attrs})
	}
	return res
}

func readVRChatExifPNG(filename string, jsonOut, rawOut, pretty, noEscape, verbose, noHuman bool) (map[string]interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ファイル読み込み失敗: %v\n", err)
		return nil, err
	}

	// suppress unused parameter warnings for flags currently not used
	_ = pretty
	_ = noEscape

	// Human-readable output target:
	// - default: stdout
	// - when --json: stderr (so stdout stays pure JSON)
	// - when --no-human: suppressed (io.Discard)
	var humanOut io.Writer = os.Stdout
	if jsonOut {
		humanOut = os.Stderr
	}
	if noHuman {
		humanOut = io.Discard
	}

	// Print basic file info
	ft := detectFileType(data)
	fmt.Fprintf(humanOut, "FileType: %s\n", ft)
	switch ft {
	case "PNG":
		if w, h, err := extractPNGDimensions(data); err == nil {
			fmt.Fprintf(humanOut, "ImageWidth: %dpx\n", w)
			fmt.Fprintf(humanOut, "ImageHeight: %dpx\n", h)
		}
	case "WebP":
		if w, h, hasAlpha, hasAnim, err := extractWebPDimensionsAndFlags(data); err == nil {
			fmt.Fprintf(humanOut, "ImageWidth: %dpx\n", w)
			fmt.Fprintf(humanOut, "ImageHeight: %dpx\n", h)
			fmt.Fprintf(humanOut, "Alpha: %v\n", map[bool]string{true: "Yes", false: "No"}[hasAlpha])
			fmt.Fprintf(humanOut, "Animation: %v\n", map[bool]string{true: "Yes", false: "No"}[hasAnim])
		} else {
			// still show presence of animation/alpha if chunks exist
			if _, _, hasAlpha, hasAnim, _ := extractWebPDimensionsAndFlags(data); hasAlpha || hasAnim {
				fmt.Fprintf(humanOut, "Alpha: %v\n", map[bool]string{true: "Yes", false: "No"}[hasAlpha])
				fmt.Fprintf(humanOut, "Animation: %v\n", map[bool]string{true: "Yes", false: "No"}[hasAnim])
			}
		}
	}

	// Collect metadata for JSON/raw modes
	meta := map[string]interface{}{"fileName": filename, "fileType": ft}
	switch ft {
	case "PNG":
		if w, h, err := extractPNGDimensions(data); err == nil {
			meta["imageWidth"] = w
			meta["imageHeight"] = h
		}
	case "WebP":
		if w, h, hasAlpha, hasAnim, err := extractWebPDimensionsAndFlags(data); err == nil {
			meta["imageWidth"] = w
			meta["imageHeight"] = h
			meta["alpha"] = hasAlpha
			meta["animation"] = hasAnim
		} else {
			if _, _, hasAlpha, hasAnim, _ := extractWebPDimensionsAndFlags(data); hasAlpha || hasAnim {
				meta["alpha"] = hasAlpha
				meta["animation"] = hasAnim
			}
		}
	}

	// EXIF
	rawExif, _ := exif.SearchAndExtractExif(data)
	if rawExif == nil {
		if r, err := extractExifFromPNG(data); err == nil {
			rawExif = r
		}
	}
	if rawExif == nil {
		if r, err := extractExifFromWebP(data); err == nil {
			rawExif = r
		}
	}
	if rawExif != nil {
		meta["exifRawBase64"] = base64.StdEncoding.EncodeToString(rawExif)
		if entries, _, err := exif.GetFlatExifData(rawExif, nil); err == nil {
			var earr []map[string]string
			for _, entry := range entries {
				var val string
				switch v := entry.Value.(type) {
				case []byte:
					val = string(v)
				case string:
					val = v
				default:
					val = fmt.Sprintf("%v", v)
				}
				earr = append(earr, map[string]string{"tag": entry.TagName, "value": val})
			}
			meta["exifEntries"] = earr
		}
	}

	// XMP (PNG)
	if t, err := extractTextualMetadataFromPNG(data); err == nil {
		meta["xmpRawPNG"] = t
		var all []map[string]interface{}
		for _, f := range extractAllXMPFields(t) {
			m := map[string]interface{}{"ns": f.NS, "name": f.Name, "value": f.Value}
			if len(f.Attrs) > 0 {
				am := map[string]string{}
				for _, a := range f.Attrs {
					am[a.Name.Local] = a.Value
				}
				m["attrs"] = am
			}
			all = append(all, m)
		}
		meta["xmpFieldsPNG"] = all
	}
	// XMP (WebP)
	if t2, err := extractTextualMetadataFromWebP(data); err == nil {
		meta["xmpRawWebP"] = t2
		var all []map[string]interface{}
		for _, f := range extractAllXMPFields(t2) {
			m := map[string]interface{}{"ns": f.NS, "name": f.Name, "value": f.Value}
			if len(f.Attrs) > 0 {
				am := map[string]string{}
				for _, a := range f.Attrs {
					am[a.Name.Local] = a.Value
				}
				m["attrs"] = am
			}
			all = append(all, m)
		}
		meta["xmpFieldsWebP"] = all
	}

	// Raw debug mode
	if rawOut {
		if jsonOut {
			fmt.Fprintln(os.Stderr, "--- RAW DATA (debug) ---")
			if v, ok := meta["exifRawBase64"]; ok {
				fmt.Fprintf(os.Stderr, "EXIF (base64): %s\n", v)
			}
			if v, ok := meta["xmpRawPNG"]; ok {
				fmt.Fprintln(os.Stderr, "XMP (PNG):")
				fmt.Fprintln(os.Stderr, v)
			}
			if v, ok := meta["xmpRawWebP"]; ok {
				fmt.Fprintln(os.Stderr, "XMP (WebP):")
				fmt.Fprintln(os.Stderr, v)
			}
			// do not exit - allow caller to handle JSON output
		} else {
			fmt.Fprintln(humanOut, "--- RAW DATA (debug) ---")
			if v, ok := meta["exifRawBase64"]; ok {
				fmt.Fprintf(humanOut, "EXIF (base64): %s\n", v)
			}
			if v, ok := meta["xmpRawPNG"]; ok {
				fmt.Fprintln(humanOut, "XMP (PNG):")
				fmt.Fprintln(humanOut, v)
			}
			if v, ok := meta["xmpRawWebP"]; ok {
				fmt.Fprintln(humanOut, "XMP (WebP):")
				fmt.Fprintln(humanOut, v)
			}
		}
	}

	// If caller wants JSON, return the meta and let caller serialize
	// unless verbose was requested: in verbose+json mode we continue and
	// emit human-readable diagnostics to the humanOut (stderr by default).
	if jsonOut && !verbose {
		return meta, nil
	}

	rawExif, err = exif.SearchAndExtractExif(data)
	if err != nil {
		// Try PNG eXIf chunk as a fallback
		rawExif, err = extractExifFromPNG(data)
		if err != nil {
			// Try WebP EXIF chunk as a fallback
			rawExif, err = extractExifFromWebP(data)
		}
	}

	// If still no EXIF, try textual metadata (PNG then WebP)
	if rawExif == nil {
		// Try PNG textual
		text, terr := extractTextualMetadataFromPNG(data)
		if terr == nil {
			// Try parsing XMP within textual metadata
			if ok, wid, wname, aid := extractVRChatFromXMP(text); ok {
				fmt.Fprintf(humanOut, "ワールドID: %s\n", wid)
				if wname != "" {
					fmt.Fprintf(humanOut, "ワールド名: %s\n", wname)
				}
				if aid != "" {
					fmt.Fprintf(humanOut, "撮影者ID: %s\n", aid)
				}
				fmt.Fprintf(humanOut, "URL: https://vrchat.com/home/launch?worldId=%s\n", wid)
				return meta, nil
			}
			fmt.Fprintf(humanOut, "詳細情報: %s\n", text)
			parts := strings.LastIndex(text, " wrld_")
			if parts != -1 {
				worldName := strings.Trim(text[:parts], "\"")
				worldID := strings.Trim(text[parts+1:], "\"")
				fmt.Fprintf(humanOut, "ワールド名: %s\n", worldName)
				fmt.Fprintf(humanOut, "ワールドID: %s\n", worldID)
				fmt.Fprintf(humanOut, "URL: https://vrchat.com/home/launch?worldId=%s\n", worldID)
			}
			return meta, nil
		}

		// Try WebP XMP chunk
		text2, terr2 := extractTextualMetadataFromWebP(data)
		if terr2 == nil { // Try parsing XMP
			if ok, wid, wname, aid := extractVRChatFromXMP(text2); ok {
				fmt.Fprintf(humanOut, "ワールドID: %s\n", wid)
				if wname != "" {
					fmt.Fprintf(humanOut, "ワールド名: %s\n", wname)
				}
				if aid != "" {
					fmt.Fprintf(humanOut, "撮影者ID: %s\n", aid)
				}
				fmt.Fprintf(humanOut, "URL: https://vrchat.com/home/launch?worldId=%s\n", wid)
			}
			// Print all XMP fields
			for _, f := range extractAllXMPFields(text2) {
				fmt.Fprintf(humanOut, "%s (%s): %s\n", f.Name, f.NS, f.Value)
				for _, a := range f.Attrs {
					fmt.Fprintf(humanOut, "  @%s=%s\n", a.Name.Local, a.Value)
				}
			}
			return meta, nil
		}

		fmt.Fprintf(humanOut, "Exifが見つかりません: %v\n", err)
		return meta, nil
	}
	// 第2引数にnilを渡すと、デフォルトのタグインデックスが使用されます
	entries, _, err := exif.GetFlatExifData(rawExif, nil)
	if err != nil {
		fmt.Fprintf(humanOut, "Exif解析失敗: %v\n", err)
		return meta, nil
	}

	for _, entry := range entries {
		// Exif タグの値を文字列として取得します
		var val string
		switch v := entry.Value.(type) {
		case []byte:
			val = string(v)
		case string:
			val = v
		default:
			val = fmt.Sprintf("%v", v)
		}

		switch entry.TagName {
		case "DateTime":
			fmt.Fprintf(humanOut, "撮影日時: %s\n", val)
		case "ImageDescription":
			fmt.Fprintf(humanOut, "詳細情報: %s\n", val)

			parts := strings.LastIndex(val, " wrld_")
			if parts != -1 {
				worldName := strings.Trim(val[:parts], "\"")
				worldID := strings.Trim(val[parts+1:], "\"")
				fmt.Fprintf(humanOut, "ワールド名: %s\n", worldName)
				fmt.Fprintf(humanOut, "ワールドID: %s\n", worldID)
				fmt.Fprintf(humanOut, "URL: https://vrchat.com/home/launch?worldId=%s\n", worldID)
			}
		case "Artist":
			fmt.Fprintf(humanOut, "撮影者: %s\n", val)
		}
	}
	// Also attempt to extract XMP metadata (PNG/WebP) and print all fields
	if text, terr := extractTextualMetadataFromPNG(data); terr == nil {
		fmt.Println("--- XMP (PNG) ---")
		for _, f := range extractAllXMPFields(text) {
			fmt.Fprintf(humanOut, "%s (%s): %s\n", f.Name, f.NS, f.Value)
			for _, a := range f.Attrs {
				fmt.Fprintf(humanOut, "  @%s=%s\n", a.Name.Local, a.Value)
			}
		}
	}
	if text2, terr2 := extractTextualMetadataFromWebP(data); terr2 == nil {
		fmt.Println("--- XMP (WebP) ---")
		for _, f := range extractAllXMPFields(text2) {
			fmt.Fprintf(humanOut, "%s (%s): %s\n", f.Name, f.NS, f.Value)
			for _, a := range f.Attrs {
				fmt.Fprintf(humanOut, "  @%s=%s\n", a.Name.Local, a.Value)
			}
		}
	}
	return meta, nil
}
