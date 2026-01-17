package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "image/jpeg"

	"github.com/chai2010/webp"
	_ "golang.org/x/image/webp"
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

// XMPから撮影日を取得する
func extractDateFromXMP(xmp string) string {
	dec := xml.NewDecoder(strings.NewReader(xmp))
	const xmpNS = "http://ns.adobe.com/xap/1.0/"
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
			if se.Name.Space == xmpNS && se.Name.Local == "CreateDate" {
				var v string
				_ = dec.DecodeElement(&v, &se)
				if v != "" {
					return strings.TrimSpace(v)
				}
			}
		}
	}
	return ""
}

// XMPから作者名を取得する
func extractAuthorFromXMP(xmp string) string {
	dec := xml.NewDecoder(strings.NewReader(xmp))
	const xmpNS = "http://ns.adobe.com/xap/1.0/"
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
			if se.Name.Space == xmpNS && se.Name.Local == "Author" {
				var v string
				_ = dec.DecodeElement(&v, &se)
				if v != "" {
					return strings.TrimSpace(v)
				}
			}
		}
	}
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

	// Try XMP (PNG)
	if t, err := extractTextualMetadataFromPNG(data); err == nil {
		meta["xmpRawPNG"] = t

		// VRChat用メタデータも抽出
		if ok, wid, wname, aid := extractVRChatFromXMP(t); ok {
			meta["worldID"] = wid
			meta["worldName"] = wname
			meta["authorID"] = aid
		}
		// 撮影日・作者名も抽出
		shootDate := extractDateFromXMP(t)
		if shootDate != "" {
			meta["shootDate"] = shootDate
		}
		authorName := extractAuthorFromXMP(t)
		if authorName != "" {
			meta["authorName"] = authorName
		}
	}
	
	// Try XMP (WebP)
	if t2, err := extractTextualMetadataFromWebP(data); err == nil {
		meta["xmpRawWebP"] = t2

		// Extract VRChat-specific metadata from WebP XMP
		if ok, wid, wname, aid := extractVRChatFromXMP(t2); ok {
			meta["worldID"] = wid
			meta["worldName"] = wname
			meta["authorID"] = aid
		}
		
		// Extract shoot date and author name
		shootDate := extractDateFromXMP(t2)
		if shootDate != "" {
			meta["shootDate"] = shootDate
		}
		
		authorName := extractAuthorFromXMP(t2)
		if authorName != "" {
			meta["authorName"] = authorName
		}
	}

	return meta, nil
}

func isDarkImage(img image.Image) bool {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// サンプリング: 全体の約10%を確認
	sampleStep := 1
	if w > 100 || h > 100 {
		sampleStep = (w + 99) / 100
	}

	var totalBrightness float64
	sampleCount := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleStep {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleStep {
			r, g, b, _ := img.At(x, y).RGBA()
			// RGBA returns 16-bit values
			brightness := float64(r+g+b) / 3.0 / 65535.0
			totalBrightness += brightness
			sampleCount++
		}
	}

	if sampleCount == 0 {
		return false
	}

	averageBrightness := totalBrightness / float64(sampleCount)
	return averageBrightness < 0.5 // 50%を閾値とする
}

func addMetadataToImage(imagePath string, date string, worldName string, authorName string, authorID string, worldURL string) error {
	// 画像を読み込む
	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 画像をデコード
	img, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// プリントカメラ解像度判定
	if isPrintCameraResolutionOnly(img) {
		if worldURL == "" {
			// ワールド情報なし → 元画像をそのまま保存
			outputDir := filepath.Join(filepath.Dir(imagePath), "annotated")
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return err
			}
			outputPath := filepath.Join(outputDir, filepath.Base(imagePath))
			
			// 元画像をコピー
			origData, err := os.ReadFile(imagePath)
			if err != nil {
				return err
			}
			return os.WriteFile(outputPath, origData, 0644)
		}
		
		// ワールド情報あり → rMQRコードのみ白背景で右上に描画
		outImg := image.NewRGBA(bounds)
		draw.Draw(outImg, bounds, img, bounds.Min, draw.Src)
		// TODO: rMQRコード生成と描画
		
		outputDir := filepath.Join(filepath.Dir(imagePath), "annotated")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}
		outputPath := filepath.Join(outputDir, filepath.Base(imagePath))
		isWebP := strings.HasSuffix(strings.ToLower(imagePath), ".webp")
		
		if isWebP {
			if !strings.HasSuffix(strings.ToLower(outputPath), ".webp") {
				outputPath = outputPath + ".webp"
			}
			var buf bytes.Buffer
			err = webp.Encode(&buf, outImg, &webp.Options{Lossless: true, Quality: 100})
			if err != nil {
				return err
			}
			
			outFile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer outFile.Close()
			_, err = outFile.Write(buf.Bytes())
			return err
		} else {
			if strings.HasSuffix(strings.ToLower(outputPath), ".webp") {
				outputPath = outputPath[:len(outputPath)-5] + ".png"
			}
			outFile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer outFile.Close()
			return png.Encode(outFile, outImg)
		}
	}

	// 通常処理（余白・テキスト・QR）
	marginTop := 69
	newWidth := width
	newHeight := height + marginTop
	var bgColor color.Color
	if isDarkImage(img) {
		bgColor = color.Black
	} else {
		bgColor = color.White
	}
	newImg := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.Draw(newImg, newImg.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)
	draw.Draw(newImg, image.Rect(0, marginTop, width, marginTop+height), img, bounds.Min, draw.Over)
	
	if worldName == "" {
		if date == "" {
			date = extractDateFromFilename(imagePath)
		}
		worldURL = ""
	}
	
	// TODO: addTextToImage の呼び出し
	
	outputDir := filepath.Join(filepath.Dir(imagePath), "annotated")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	outputPath := filepath.Join(outputDir, filepath.Base(imagePath))

	// 拡張子判定
	isWebP := strings.HasSuffix(strings.ToLower(imagePath), ".webp")
	if isWebP {
		if !strings.HasSuffix(strings.ToLower(outputPath), ".webp") {
			outputPath = outputPath + ".webp"
		}
		var buf bytes.Buffer
		err = webp.Encode(&buf, newImg, &webp.Options{Lossless: true, Quality: 100})
		if err != nil {
			return err
		}
		
		outFile, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer outFile.Close()
		_, err = outFile.Write(buf.Bytes())
		return err
	} else {
		if strings.HasSuffix(strings.ToLower(outputPath), ".webp") {
			outputPath = outputPath[:len(outputPath)-5] + ".png"
		}
		outFile, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer outFile.Close()
		return png.Encode(outFile, newImg)
	}
}

func extractDateFromFilename(filePath string) string {
	filename := filepath.Base(filePath)
	
	// パターン1: VRChat_2026-01-15_22-52-38.319_3840x2160
	// パターン2: VRChat_2026-01-14_21-49-03.450_2048x1440
	// パターン3: VRChat_1920x1080_2022-06-02_03-11-38.751
	re1 := regexp.MustCompile(`VRChat_(?:\d+x\d+_)?(\d{4}-\d{2}-\d{2})_(\d{2}-\d{2}-\d{2})`)
	if matches := re1.FindStringSubmatch(filename); len(matches) > 2 {
		return matches[1] + "T" + strings.ReplaceAll(matches[2], "-", ":")
	}

	// パターン: com.vrchat.oculus.quest-20220330-003003
	re2 := regexp.MustCompile(`-(\d{8})-(\d{6})`)
	if matches := re2.FindStringSubmatch(filename); len(matches) > 2 {
		dateStr := matches[1]
		timeStr := matches[2]
		return dateStr[0:4] + "-" + dateStr[4:6] + "-" + dateStr[6:8] + "T" + 
			timeStr[0:2] + ":" + timeStr[2:4] + ":" + timeStr[4:6]
	}
	
	return ""
}

func formatDateAsYMD(dateStr string) string {
	// ISO 8601 形式を解析: "2026-01-15T09:38:22.0000000+09:00"
	// フォーマット: "2026-01-15 WED 09:38:22"
	
	if len(dateStr) >= 19 {
		// 日付を解析
		year := dateStr[0:4]
		month := dateStr[5:7]
		day := dateStr[8:10]
		hour := dateStr[11:13]
		minute := dateStr[14:16]
		second := dateStr[17:19]
		
		// 曜日を計算
		t, err := time.Parse("2006-01-02", dateStr[0:10])
		var dayOfWeek string
		if err == nil {
			dayOfWeek = t.Format("Mon")
			dayOfWeek = strings.ToUpper(dayOfWeek)
		} else {
			dayOfWeek = "???"
		}
		
		return fmt.Sprintf("%s-%s-%s %s %s:%s:%s", year, month, day, dayOfWeek, hour, minute, second)
	}
	return dateStr
}

func generateRMQR(url string, isDark bool) (image.Image, error) {
	return nil, errors.New("not implemented")
}

func invertImage(img image.Image) image.Image {
	return img
}
