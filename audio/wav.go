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
	file        *os.File
	bytesBuffer []byte
	dataOffset  int64
}

// NewWAVFormat opens a WAV file and reads its header.
// file is left open
func NewWAVFormat(path string) (*WAVFormat, error) {

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	wav := &WAVFormat{file: file, bytesBuffer: make([]byte, 4096)}
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

	if err := binary.Read(w.file, binary.LittleEndian, &w.ChunkSize); err != nil {
		return fmt.Errorf("error reading ChunkSize: %w", err)
	}

	if err := binary.Read(w.file, binary.LittleEndian, &w.Format); err != nil {
		return fmt.Errorf("error reading Format: %w", err)
	}
	if string(w.Format[:]) != "WAVE" {
		return fmt.Errorf("not a valid WAVE file")
	}

	var foundFmt, foundData bool
	for !foundFmt || !foundData {
		var chunkID [4]byte
		var chunkSize uint32
		if err := binary.Read(w.file, binary.LittleEndian, &chunkID); err != nil {
			return fmt.Errorf("error reading chunk ID: %w", err)
		}
		if err := binary.Read(w.file, binary.LittleEndian, &chunkSize); err != nil {
			return fmt.Errorf("error reading chunk size: %w", err)
		}

		switch string(chunkID[:]) {
		case "fmt ":
			foundFmt = true
			w.Subchunk1ID = chunkID
			w.Subchunk1Size = chunkSize
			// read fmt fields
			if err := binary.Read(w.file, binary.LittleEndian, &w.AudioFormat); err != nil {
				return fmt.Errorf("error reading AudioFormat: %w", err)
			}
			if w.AudioFormat != 1 && w.AudioFormat != 0xFFFE {
				return fmt.Errorf("unsupported audio format: %d (only PCM and EXTENSIBLE are supported)", w.AudioFormat)
			}
			if err := binary.Read(w.file, binary.LittleEndian, &w.NumChannels); err != nil {
				return fmt.Errorf("error reading NumChannels: %w", err)
			}
			if err := binary.Read(w.file, binary.LittleEndian, &w.Samplerate); err != nil {
				return fmt.Errorf("error reading Samplerate: %w", err)
			}
			if err := binary.Read(w.file, binary.LittleEndian, &w.ByteRate); err != nil {
				return fmt.Errorf("error reading ByteRate: %w", err)
			}
			if err := binary.Read(w.file, binary.LittleEndian, &w.BlockAlign); err != nil {
				return fmt.Errorf("error reading BlockAlign: %w", err)
			}
			if err := binary.Read(w.file, binary.LittleEndian, &w.BitsPerSample); err != nil {
				return fmt.Errorf("error reading BitsPerSample: %w", err)
			}

			// skip extra fmt bytes if Subchunk1Size is larger than 16

			if w.Subchunk1Size > 16 {
				if w.AudioFormat == 0xFFFE {
					var cbSize, validBits uint16
					var chanMask uint32
					var subFormat [16]byte

					if err := binary.Read(w.file, binary.LittleEndian, &cbSize); err != nil {
						return fmt.Errorf("error reading EXTENSIBLE cbSize: %w", err)
					}
					if err := binary.Read(w.file, binary.LittleEndian, &validBits); err != nil {
						return fmt.Errorf("error reading valid bits per sample: %w", err)
					}
					if err := binary.Read(w.file, binary.LittleEndian, &chanMask); err != nil {
						return fmt.Errorf("error reading channel mask: %w", err)
					}
					if err := binary.Read(w.file, binary.LittleEndian, &subFormat); err != nil {
						return fmt.Errorf("error reading subformat GUID: %w", err)
					}
					if validBits > 0 && validBits <= 32 {
						w.BitsPerSample = validBits
					}
					// Skip any bytes beyond the EXTENSIBLE fields
					extra := int64(w.Subchunk1Size) - 16 - 24
					if extra > 0 {
						_, err := w.file.Seek(extra, io.SeekCurrent)
						if err != nil {
							return fmt.Errorf("error skipping extra fmt bytes: %w", err)
						}
					}
				} else {
					if _, err := w.file.Seek(int64(w.Subchunk1Size-16), io.SeekCurrent); err != nil {
						return fmt.Errorf("error skipping extra fmt bytes: %w", err)
					}
				}
			}

		case "data":
			foundData = true
			w.Subchunk2ID = chunkID
			w.Subchunk2Size = chunkSize

			// store the offset where audio data begins
			dataOffset, err := w.file.Seek(0, io.SeekCurrent)
			if err != nil {
				return fmt.Errorf("error getting data offset: %w", err)
			}
			w.dataOffset = dataOffset

		default:
			// skip unknown chunk
			// pads if odd size
			skipSize := int64(chunkSize)
			if chunkSize%2 != 0 {
				skipSize++
			}
			if _, err := w.file.Seek(skipSize, io.SeekCurrent); err != nil {
				return fmt.Errorf("error skipping unknown chunk: %w", err)
			}
		}

	}

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
	if len(buffer) == 0 {
		return 0, nil
	}

	// Calculate the number of bytes per sample based on bit depth.
	bytesPerSample := w.BitDepth() / 8
	samplesRead := 0

	needed := len(buffer) * bytesPerSample
	if cap(w.bytesBuffer) < needed {
		w.bytesBuffer = make([]byte, needed)
	}
	bytesBuffer := w.bytesBuffer[:needed]

	n, err := io.ReadFull(w.file, bytesBuffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return 0, fmt.Errorf("error reading audio data: %w", err)
	}
	if n == 0 {
		return 0, io.EOF
	}
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	if n%bytesPerSample != 0 {
		return 0, fmt.Errorf("corrupt input: %d bytes is not aligned to %d-byte sample size", n, bytesPerSample)
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
	if len(bytes) < w.BitDepth()/8 {
		return 0
	}

	switch w.BitDepth() {
	case 8:
		// convert the byte directly and adjust for unsigned range.
		return int32(bytes[0]) - 128
	case 16:
		// convert the byte slice to a 16-bit integer.
		return int32(int16(bytes[0]) | int16(bytes[1])<<8)
	case 24:
		// manually construct the 32-bit integer and handle sign extension.
		sample := int32(bytes[0]) | int32(bytes[1])<<8 | int32(bytes[2])<<16
		if sample&0x800000 != 0 {
			sample |= ^0xffffff // Sign extension for negative values.
		}
		return sample
	case 32:
		// convert the byte slice to a 32-bit integer.
		return int32(bytes[0]) | int32(bytes[1])<<8 | int32(bytes[2])<<16 | int32(bytes[3])<<24
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
