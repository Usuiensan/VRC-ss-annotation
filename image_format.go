package main

import (
	"bytes"
	"encoding/binary"
	"errors"
)

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
			return int(binary.BigEndian.Uint32(data[chunkDataStart : chunkDataStart+4])), int(binary.BigEndian.Uint32(data[chunkDataStart+4 : chunkDataStart+8])), nil
		}
		offset = chunkCRCEnd
	}
	return 0, 0, errors.New("IHDR not found")
}

func parseLittle24(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }

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
				width = parseLittle24(b[4:7]) + 1
				height = parseLittle24(b[7:10]) + 1
			}
		case "ALPH":
			hasAlpha = true
		case "ANIM":
			hasAnim = true
		case "VP8 ":
			if size >= 10 {
				b := data[chunkDataStart:chunkDataEnd]
				w := int(binary.LittleEndian.Uint16(b[6:8]))
				h := int(binary.LittleEndian.Uint16(b[8:10]))
				if w != 0 && h != 0 {
					width, height = w, h
				}
			}
		case "VP8L":
			if size >= 5 {
				b := data[chunkDataStart:chunkDataEnd]
				packed := uint32(b[1]) | uint32(b[2])<<8 | uint32(b[3])<<16 | uint32(b[4])<<24
				w := int((packed & 0x3FFF) + 1)
				h := int(((packed >> 14) & 0x3FFF) + 1)
				if w != 0 && h != 0 {
					width, height = w, h
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
