package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"os"
	"sync"
	"time"

	_ "image/jpeg"
)

var logMutex sync.Mutex

func appendLog(message string) {
	logMutex.Lock()
	defer logMutex.Unlock()
	logPath := "annotated/annotate.log"
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		now := time.Now().Format("2006-01-02 15:04:05")
		f.WriteString("[" + now + "] " + message + "\n")
	}
}

// 指定解像度(2048x1440)か判定
func isPrintCameraResolutionOnly(img image.Image) bool {
	bounds := img.Bounds()
	return bounds.Dx() == 2048 && bounds.Dy() == 1440
}

func main() {
	// CLI flags
	jsonOut := flag.Bool("json", false, "出力をJSONにします")       // --json
	rawOut := flag.Bool("raw", false, "デバッグ用に生のメタデータを表示します") // --raw
	pretty := flag.Bool("pretty", false, "JSONを整形して出力します ( --json と併用 )")
	noEscape := flag.Bool("no-escape", false, "JSON出力時にHTMLエスケープを無効化します（危険）")
	ndjson := flag.Bool("ndjson", false, "JSON出力を1行ごとのNDJSONで出力します（--json と併用）")
	verbose := flag.Bool("verbose", false, "詳細な人間向け出力を有効化します（--json時はstderrに出力）")
	noHuman := flag.Bool("no-human", false, "人間向け出力を全て抑制します（--jsonと併用して純粋なJSONのみ出力する）")
	annotate := flag.Bool("annotate", false, "メタデータを画像に追加して出力します")
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
	if *annotate {
		for _, path := range flag.Args() {
			meta, err := readVRChatExifPNG(path, true, false, false, false, false, true)
			if err != nil {
				msg := fmt.Sprintf("エラー (%s): %v", path, err)
				fmt.Fprintln(os.Stderr, msg)
				appendLog(msg)
				continue
			}
			date, _ := meta["shootDate"].(string)
			worldName, _ := meta["worldName"].(string)
			worldID, _ := meta["worldID"].(string)
			authorName, _ := meta["authorName"].(string)
			authorID, _ := meta["authorID"].(string)
			var worldURL string
			if worldID == "" {
				msg := fmt.Sprintf("警告 (%s): ワールドIDが見つかりません（日時のみ表示）", path)
				fmt.Fprintln(os.Stderr, msg)
				appendLog(msg)
				worldURL = ""
			} else {
				worldURL = fmt.Sprintf("https://vrchat.com/home/world/%s", worldID)
			}
			// 2048x1440判定
			imgFile, err := os.Open(path)
			if err == nil {
				img, _, err := image.Decode(imgFile)
				imgFile.Close()
				if err == nil && isPrintCameraResolutionOnly(img) {
					msg := fmt.Sprintf("%s: 2048x1440画像のため撮影者・ワールド名・撮影日を記載しません", path)
					fmt.Println(msg)
					appendLog(msg)
				}
			}
			if err := addMetadataToImage(path, date, worldName, authorName, authorID, worldURL); err != nil {
				msg := fmt.Sprintf("画像処理エラー (%s): %v", path, err)
				fmt.Fprintln(os.Stderr, msg)
				appendLog(msg)
				continue
			}
			msg := fmt.Sprintf("処理完了: %s", path)
			fmt.Println(msg)
			appendLog(msg)
		}
		return
	}

	for _, path := range flag.Args() {
		fmt.Printf("\n--- ファイル: %s ---\n", path)
		_, _ = readVRChatExifPNG(path, *jsonOut, *rawOut, *pretty, *noEscape, *verbose, *noHuman)
	}

	if !*jsonOut && !*rawOut && !*annotate {
		fmt.Println("\n数秒後に自動で終了します...")
		time.Sleep(3 * time.Second)
	}
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
			if size >= 10 {
				b := data[chunkDataStart:chunkDataEnd]
				if len(b) >= 10 {
					w := int(binary.LittleEndian.Uint16(b[6:8]))
					h := int(binary.LittleEndian.Uint16(b[8:10]))
					if w != 0 && h != 0 {
						width = w
						height = h
					}
				}
			}
		case "VP8L":
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

// プレースホルダー関数（後で実装）
func extractExifFromPNG(data []byte) ([]byte, error) {
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
		case "iTXt", "zTXt":
			// Other text formats - skip for now
		}

		offset = chunkCRCEnd
	}

	return "", errors.New("textual metadata not found")
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
	return false, "", "", ""
}

func extractDateFromXMP(xmp string) string {
	return ""
}

func extractAuthorFromXMP(xmp string) string {
	return ""
}

func readVRChatExifPNG(filename string, jsonOut, rawOut, pretty, noEscape, verbose, noHuman bool) (map[string]interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ファイル読み込み失敗: %v\n", err)
		return nil, err
	}

	var humanOut io.Writer = os.Stdout
	if jsonOut {
		humanOut = os.Stderr
	}
	if noHuman {
		humanOut = io.Discard
	}

	ft := detectFileType(data)
	fmt.Fprintf(humanOut, "FileType: %s\n", ft)

	meta := map[string]interface{}{"fileName": filename, "fileType": ft}

	switch ft {
	case "PNG":
		if w, h, err := extractPNGDimensions(data); err == nil {
			fmt.Fprintf(humanOut, "ImageWidth: %dpx\n", w)
			fmt.Fprintf(humanOut, "ImageHeight: %dpx\n", h)
			meta["imageWidth"] = w
			meta["imageHeight"] = h
		}
	case "WebP":
		if w, h, hasAlpha, hasAnim, err := extractWebPDimensionsAndFlags(data); err == nil {
			fmt.Fprintf(humanOut, "ImageWidth: %dpx\n", w)
			fmt.Fprintf(humanOut, "ImageHeight: %dpx\n", h)
			fmt.Fprintf(humanOut, "Alpha: %v\n", map[bool]string{true: "Yes", false: "No"}[hasAlpha])
			fmt.Fprintf(humanOut, "Animation: %v\n", map[bool]string{true: "Yes", false: "No"}[hasAnim])
			meta["imageWidth"] = w
			meta["imageHeight"] = h
			meta["alpha"] = hasAlpha
			meta["animation"] = hasAnim
		}
	}

	return meta, nil
}

func isDarkImage(img image.Image) bool {
	return false // プレースホルダー
}

func addMetadataToImage(imagePath string, date string, worldName string, authorName string, authorID string, worldURL string) error {
	return fmt.Errorf("not implemented")
}

func extractDateFromFilename(filePath string) string {
	return ""
}

func formatDateAsYMD(dateStr string) string {
	return dateStr
}

func generateRMQR(url string, isDark bool) (image.Image, error) {
	return nil, errors.New("not implemented")
}

func invertImage(img image.Image) image.Image {
	return img
}
