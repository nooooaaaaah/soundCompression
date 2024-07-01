package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const (
	WAVHeaderSize = 44
)

type WAVFormat struct {
	// RIFF chunk
	ChunkID   [4]byte // Should be "RIFF"
	ChunkSize uint32  // 4 + (8 + SubChunk1Size) + (8 + SubChunk2Size)
	Format    [4]byte // Should be "WAVE"

	// fmt sub-chunk
	Subchunk1ID   [4]byte // Should be "fmt "
	Subchunk1Size uint32  // 16 for PCM
	AudioFormat   uint16  // 1 for PCM
	NumChannels   uint16  // 1 for mono, 2 for stereo
	Samplerate    uint32  // 8000, 44100, etc.
	ByteRate      uint32  // SampleRate * NumChannels * BitsPerSample/8
	BlockAlign    uint16  // NumChannels * BitsPerSample/8
	BitsPerSample uint16  // 8 bits = 8, 16 bits = 16, etc.

	// data sub-chunk
	Subchunk2ID   [4]byte // Should be "data"
	Subchunk2Size uint32  // NumSamples * NumChannels * BitsPerSample/8

	// File handling
	file       *os.File
	dataOffset int64
}

// NewWAVFormat opens a WAV file and reads its header.
// file is left open
func NewWAVFormat(path string) (*WAVFormat, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	wav := &WAVFormat{file: file}
	if err := wav.readHeader(); err != nil {
		file.Close()
		return nil, err
	}

	return wav, nil
}

// readHeader reads and validates the WAV file header.
func (w *WAVFormat) readHeader() error {
	// Read RIFF chunk
	if err := binary.Read(w.file, binary.LittleEndian, &w.ChunkID); err != nil {
		return fmt.Errorf("error reading ChunkID: %w", err)
	}
	if string(w.ChunkID[:]) != "RIFF" {
		return fmt.Errorf("not a valid RIFF file")
	}

	headerFields := []any{
		&w.ChunkSize, &w.Format,
		&w.Subchunk1ID, &w.Subchunk1Size, &w.AudioFormat,
		&w.NumChannels, &w.Samplerate, &w.ByteRate,
		&w.BlockAlign, &w.BitsPerSample,
		&w.Subchunk2ID, &w.Subchunk2Size,
	}

	for _, field := range headerFields {
		if err := binary.Read(w.file, binary.LittleEndian, field); err != nil {
			return fmt.Errorf("error reading WAV header: %w", err)
		}
	}

	if string(w.Format[:]) != "WAVE" {
		return fmt.Errorf("not a valid WAVE file")
	}
	if string(w.Subchunk1ID[:]) != "fmt " {
		return fmt.Errorf("fmt sub-chunk not found")
	}
	if string(w.Subchunk2ID[:]) != "data" {
		return fmt.Errorf("data sub-chunk not found")
	}

	// Store the offset where the audio data begins
	dataOffset, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("error getting data offset: %w", err)
	}
	w.dataOffset = dataOffset

	return nil
}

// SampleRate returns the sample rate of the WAV file.
func (w *WAVFormat) SampleRate() int {
	return int(w.Samplerate)
}

// Channels returns the number of audio channels in the WAV file.
func (w *WAVFormat) Channels() int {
	return int(w.NumChannels)
}

// BitDepth returns the bit depth of the WAV file.
func (w *WAVFormat) BitDepth() int {
	return int(w.BitsPerSample)
}

// TotalSamples returns the total number of audio samples in the WAV file.
func (w *WAVFormat) TotalSamples() uint64 {
	return uint64(w.Subchunk2Size) / uint64(w.BlockAlign)
}

// ReadSamples reads audio samples into the provided buffer.
func (w *WAVFormat) ReadSamples(buffer []int32) (int, error) {
	// Calculate the number of bytes per sample based on bit depth.
	bytesPerSample := w.BitDepth() / 8
	samplesRead := 0

	bytesBuffer := make([]byte, len(buffer)*bytesPerSample)

	n, err := w.file.Read(bytesBuffer)
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("error reading audio data: %w", err)
	}

	samplesRead = n / bytesPerSample

	// Convert the raw byte data into 32-bit integer samples.
	for i := 0; i < samplesRead; i++ {
		sampleBytes := bytesBuffer[i*bytesPerSample : (i+1)*bytesPerSample]
		buffer[i] = w.bytesToInt32(sampleBytes)
	}

	return samplesRead, nil
}

// bytesToInt32 converts a byte slice to a 32-bit integer based on the bit depth.
func (w *WAVFormat) bytesToInt32(bytes []byte) int32 {
	switch w.BitDepth() {
	case 8:
		// convert the byte directly and adjust for unsigned range.
		return int32(bytes[0]) - 128
	case 16:
		// convert the byte slice to a 16-bit integer.
		return int32(int16(binary.LittleEndian.Uint16(bytes)))
	case 24:
		// manually construct the 32-bit integer and handle sign extension.
		sample := int32(bytes[0]) | int32(bytes[1])<<8 | int32(bytes[2])<<16
		if sample&0x800000 != 0 {
			sample |= ^0xffffff // Sign extension for negative values.
		}
		return sample
	case 32:
		// convert the byte slice to a 32-bit integer.
		return int32(binary.LittleEndian.Uint32(bytes))
	default:
		// Return 0 for unsupported bit depths.
		return 0
	}
}

// Close closes the WAV file.
func (w *WAVFormat) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
