package voice

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func WriteWAV(path string, samples []int16) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create WAV: %w", err)
	}
	defer file.Close()
	if err := EncodeWAV(file, samples); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close WAV: %w", err)
	}
	return nil
}

func EncodeWAV(writer io.Writer, samples []int16) error {
	dataBytes := uint32(len(samples) * 2)
	byteRate := uint32(SampleRate * channels * bitsPerSample / 8)
	blockAlign := uint16(channels * bitsPerSample / 8)

	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], 36+dataBytes)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], channels)
	binary.LittleEndian.PutUint32(header[24:28], SampleRate)
	binary.LittleEndian.PutUint32(header[28:32], byteRate)
	binary.LittleEndian.PutUint16(header[32:34], blockAlign)
	binary.LittleEndian.PutUint16(header[34:36], bitsPerSample)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], dataBytes)
	if _, err := writer.Write(header); err != nil {
		return fmt.Errorf("write WAV header: %w", err)
	}
	if err := binary.Write(writer, binary.LittleEndian, samples); err != nil {
		return fmt.Errorf("write WAV samples: %w", err)
	}
	return nil
}
