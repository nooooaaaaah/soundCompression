package audio

import (
	"os"
	"testing"
)

// Ensure WAVFormat implements Format.
var _ Format = (*WAVFormat)(nil)

const (
	sampleWavPath              = "../testdata/sample.wav"
	nonExistentWavPath         = "../nonexistent.wav"
	expectedSampleRate  uint32 = 44100
	expectedChannels    uint16 = 2
	expectedBitDepth    uint16 = 16
	expectedChunk              = "RIFF"
	expectedFormat             = "WAVE"
	expectedSubChunkOne        = "fmt "
)

func TestNewWAVFormat(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		expectError  bool
		expectedData *WAVFormat
	}{
		{
			name:        "Existing File",
			filePath:    sampleWavPath,
			expectError: false,
			expectedData: &WAVFormat{
				Samplerate:    expectedSampleRate,
				NumChannels:   expectedChannels,
				BitsPerSample: expectedBitDepth,
				ChunkID:       [4]byte{'R', 'I', 'F', 'F'},
				Format:        [4]byte{'W', 'A', 'V', 'E'},
				Subchunk1ID:   [4]byte{'f', 'm', 't', ' '},
				Subchunk2ID:   [4]byte{'d', 'a', 't', 'a'},
			},
		},
		{
			name:         "Non-Existent File",
			filePath:     nonExistentWavPath,
			expectError:  true,
			expectedData: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := os.Stat(tt.filePath); os.IsNotExist(err) {
				if !tt.expectError {
					t.Fatalf("Couldn't find %s: %v", tt.filePath, err)
				}
			}

			wavFormat, err := NewWAVFormat(tt.filePath)
			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected an error for file %s but got none", tt.filePath)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewWAVFormat failed: %v", err)
			}

			tests := []struct {
				name     string
				got      interface{}
				expected interface{}
			}{
				{"Sample Rate", wavFormat.Samplerate, tt.expectedData.Samplerate},
				{"Channels", wavFormat.NumChannels, tt.expectedData.NumChannels},
				{"Bit Depth", wavFormat.BitsPerSample, tt.expectedData.BitsPerSample},
				{"ChunkID", string(wavFormat.ChunkID[:]), string(tt.expectedData.ChunkID[:])},
				{"Format", string(wavFormat.Format[:]), string(tt.expectedData.Format[:])},
				{"Subchunk1ID", string(wavFormat.Subchunk1ID[:]), string(tt.expectedData.Subchunk1ID[:])},
				{"Subchunk2ID", string(wavFormat.Subchunk2ID[:]), string(tt.expectedData.Subchunk2ID[:])},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					if tt.got != tt.expected {
						t.Errorf("Expected %v, got %v", tt.expected, tt.got)
					}
				})
			}

			t.Run("Additional Information", func(t *testing.T) {
				t.Logf("Sample Rate: %d", wavFormat.Samplerate)
				t.Logf("Channels: %d", wavFormat.NumChannels)
				t.Logf("Bit Depth: %d", wavFormat.BitsPerSample)
				t.Logf("Audio Format: %d", wavFormat.AudioFormat)
				t.Logf("Byte Rate: %d", wavFormat.ByteRate)
				t.Logf("Block Align: %d", wavFormat.BlockAlign)
				t.Logf("Data Size: %d bytes", wavFormat.Subchunk2Size)
			})
		})
	}
}

func TestWAVFormatGetters(t *testing.T) {
	wav, err := NewWAVFormat(sampleWavPath)
	if err != nil {
		t.Fatalf("NewWAVFormat failed: %v", err)
	}
	defer wav.Close()

	if wav.SampleRate() != int(expectedSampleRate) {
		t.Errorf("SampleRate() = %d, want %d", wav.SampleRate(), expectedSampleRate)
	}
	if wav.Channels() != int(expectedChannels) {
		t.Errorf("Channels() = %d, want %d", wav.Channels(), expectedChannels)
	}
	if wav.BitDepth() != int(expectedBitDepth) {
		t.Errorf("BitDepth() = %d, want %d", wav.BitDepth(), expectedBitDepth)
	}
	expectedTotal := uint64(wav.Subchunk2Size) / uint64(wav.BlockAlign)
	if wav.TotalSamples() != expectedTotal {
		t.Errorf("TotalSamples() = %d, want %d", wav.TotalSamples(), expectedTotal)
	}
}

func TestWAVFormatClose(t *testing.T) {
	wav, err := NewWAVFormat(sampleWavPath)
	if err != nil {
		t.Fatalf("NewWAVFormat failed: %v", err)
	}
	if err := wav.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
	if err := wav.Close(); err != nil {
		t.Errorf("second Close() returned error: %v", err)
	}
}

func TestBytesToInt32(t *testing.T) {
	tests := []struct {
		name     string
		bitDepth int
		input    []byte
		want     int32
	}{
		{"8-bit zero", 8, []byte{128}, 0},
		{"8-bit min", 8, []byte{0}, -128},
		{"8-bit max", 8, []byte{255}, 127},
		{"16-bit zero", 16, []byte{0, 0}, 0},
		{"16-bit positive", 16, []byte{0x34, 0x12}, 0x1234},
		{"16-bit negative", 16, []byte{0x00, 0x80}, -32768},
		{"24-bit zero", 24, []byte{0, 0, 0}, 0},
		{"24-bit positive", 24, []byte{0x78, 0x56, 0x34}, 0x345678},
		{"24-bit negative", 24, []byte{0x00, 0x00, 0x80}, -8388608},
		{"32-bit zero", 32, []byte{0, 0, 0, 0}, 0},
		{"32-bit positive", 32, []byte{0xef, 0xbe, 0xad, 0xde}, -559038737},
		{"32-bit negative", 32, []byte{0x00, 0x00, 0x00, 0x80}, -2147483648},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WAVFormat{BitsPerSample: uint16(tt.bitDepth)}
			got := w.bytesToInt32(tt.input)
			if got != tt.want {
				t.Errorf("bytesToInt32(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestWAVFormatReadSamples(t *testing.T) {
	wav, err := NewWAVFormat(sampleWavPath)
	if err != nil {
		t.Fatalf("NewWAVFormat failed: %v", err)
	}
	defer wav.Close()

	buf := make([]int32, 1024)
	n, err := wav.ReadSamples(buf)
	if err != nil {
		t.Fatalf("ReadSamples failed: %v", err)
	}
	if n != 1024 {
		t.Errorf("ReadSamples returned %d samples, want %d", n, 1024)
	}
	nonZero := false
	for _, s := range buf[:n] {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("ReadSamples returned all zeros")
	}

	_, err = wav.ReadSamples(nil)
	if err != nil {
		t.Errorf("ReadSamples(nil) error: %v", err)
	}

	_, err = wav.ReadSamples([]int32{})
	if err != nil {
		t.Errorf("ReadSamples([]) error: %v", err)
	}
}
