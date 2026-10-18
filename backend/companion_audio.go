package main

import (
	"encoding/binary"
	"errors"
)

const companionImportAudioBytes = 24 << 20

func companionIsAudioHeader(data []byte) bool {
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" || len(data) >= 3 && string(data[:3]) == "ID3" || len(data) >= 2 && data[0] == 0xff && data[1]&0xe0 == 0xe0
}

// Read duration from the bytes, not client metadata. A phone cannot bypass the
// recording limit with a forged duration or a renamed video file.
func companionAudioInfo(data []byte) (string, float64, error) {
	invalid := errors.New("Choose a complete WAV or MP3 recording from 1 to 180 seconds, up to 24 MB.")
	if len(data) < 44 || len(data) > companionImportAudioBytes {
		return "", 0, invalid
	}
	var mime string
	var duration float64
	if string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		mime = "audio/wav"
		if uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
			return "", 0, invalid
		}
		var sampleRate uint32
		var blockAlign uint16
		samples := -1
		for offset := 12; offset < len(data); {
			if offset+8 > len(data) {
				return "", 0, invalid
			}
			size := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
			start := offset + 8
			if size > uint64(len(data)-start) {
				return "", 0, invalid
			}
			chunk := data[start : start+int(size)]
			switch string(data[offset : offset+4]) {
			case "fmt ":
				if len(chunk) < 16 || sampleRate != 0 {
					return "", 0, invalid
				}
				format, channels := binary.LittleEndian.Uint16(chunk), binary.LittleEndian.Uint16(chunk[2:])
				sampleRate = binary.LittleEndian.Uint32(chunk[4:])
				blockAlign = binary.LittleEndian.Uint16(chunk[12:])
				bits := binary.LittleEndian.Uint16(chunk[14:])
				if (format != 1 && format != 3) || channels < 1 || channels > 2 || sampleRate < 8000 || sampleRate > 192000 || (bits != 8 && bits != 16 && bits != 24 && bits != 32) || (format == 3 && bits != 32) || blockAlign != channels*(bits/8) || binary.LittleEndian.Uint32(chunk[8:]) != sampleRate*uint32(blockAlign) {
					return "", 0, invalid
				}
			case "data":
				if samples >= 0 {
					return "", 0, invalid
				}
				samples = len(chunk)
			}
			offset = start + int(size) + int(size%2)
			if offset > len(data) {
				return "", 0, invalid
			}
		}
		if sampleRate == 0 || blockAlign == 0 || samples <= 0 || samples%int(blockAlign) != 0 {
			return "", 0, invalid
		}
		duration = float64(samples) / float64(blockAlign) / float64(sampleRate)
	} else {
		mime = "audio/mpeg"
		offset, frames := 0, 0
		if string(data[:3]) == "ID3" {
			if len(data) < 10 || data[3] < 2 || data[3] > 4 {
				return "", 0, invalid
			}
			size := 0
			for _, value := range data[6:10] {
				if value&0x80 != 0 {
					return "", 0, invalid
				}
				size = size<<7 | int(value)
			}
			offset = 10 + size
			if data[3] == 4 && data[5]&0x10 != 0 {
				offset += 10
			}
			if offset >= len(data) {
				return "", 0, invalid
			}
		}
		for offset < len(data) {
			if len(data)-offset == 128 && string(data[offset:offset+3]) == "TAG" {
				offset = len(data)
				break
			}
			if len(data)-offset < 4 {
				return "", 0, invalid
			}
			h := binary.BigEndian.Uint32(data[offset : offset+4])
			version, layer := (h>>19)&3, (h>>17)&3
			bitrateIndex, rateIndex := int((h>>12)&15), int((h>>10)&3)
			if h&0xffe00000 != 0xffe00000 || version == 1 || layer != 1 || bitrateIndex == 0 || bitrateIndex == 15 || rateIndex == 3 {
				return "", 0, invalid
			}
			rate := []int{44100, 48000, 32000}[rateIndex]
			bitrates := []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
			samples, multiplier := 1152, 144
			if version != 3 {
				rate /= 2
				if version == 0 {
					rate /= 2
				}
				bitrates = []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}
				samples, multiplier = 576, 72
			}
			length := multiplier*bitrates[bitrateIndex]*1000/rate + int((h>>9)&1)
			if length < 4 || length > len(data)-offset {
				return "", 0, invalid
			}
			duration += float64(samples) / float64(rate)
			if duration > 180 {
				return "", 0, invalid
			}
			frames++
			offset += length
		}
		if frames < 3 {
			return "", 0, invalid
		}
	}
	if duration < 1 || duration > 180 {
		return "", 0, invalid
	}
	return mime, duration, nil
}
