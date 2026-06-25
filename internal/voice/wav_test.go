package voice

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestEncodeWAV(t *testing.T) {
	samples := []int16{-32768, -1, 0, 1, 32767}
	var output bytes.Buffer
	if err := EncodeWAV(&output, samples); err != nil {
		t.Fatalf("EncodeWAV: %v", err)
	}
	data := output.Bytes()
	if got := string(data[0:4]); got != "RIFF" {
		t.Fatalf("chunk ID = %q", got)
	}
	if got := string(data[8:12]); got != "WAVE" {
		t.Fatalf("format = %q", got)
	}
	if got := binary.LittleEndian.Uint32(data[24:28]); got != SampleRate {
		t.Fatalf("sample rate = %d", got)
	}
	if got := binary.LittleEndian.Uint16(data[22:24]); got != 1 {
		t.Fatalf("channels = %d", got)
	}
	if got := binary.LittleEndian.Uint16(data[34:36]); got != 16 {
		t.Fatalf("bits per sample = %d", got)
	}
	if got, want := binary.LittleEndian.Uint32(data[40:44]), uint32(len(samples)*2); got != want {
		t.Fatalf("data length = %d, want %d", got, want)
	}
	for i, want := range samples {
		got := int16(binary.LittleEndian.Uint16(data[44+i*2:]))
		if got != want {
			t.Fatalf("sample %d = %d, want %d", i, got, want)
		}
	}
}
