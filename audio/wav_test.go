package audio

import (
	"os"
	"testing"
)

const (
	sampleWavPath              = "../sample.wav"
	nonExistentWavPath         = "../nonexistent.wav"
	expectedSampleRate  uint32 = 44100
	expectedChannels    uint16 = 2
	expectedBitDepth    uint16 = 16
	expectedChunk              = "RIFF"
	expectedFormat             = "WAVE"
	expectedSubChunkOne        = "fmt "
	expectedsubChunkTwo        = "data"
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
