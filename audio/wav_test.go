package audio

import (
	"os"
	"testing"
)

const (
	sampleWavPath              = "../sample.wav"
	expectedSampleRate  uint32 = 44100
	expectedChannels    uint16 = 2
	expectedBitDepth    uint16 = 16
	expectedChunk              = "RIFF"
	expectedFormat             = "WAVE"
	expectedSubChunkOne        = "fmt "
	expectedsubChunkTwo        = "data"
)

func TestNewWAVFormat(t *testing.T) {
	t.Run("File Existence", func(t *testing.T) {
		if _, err := os.Stat(sampleWavPath); os.IsNotExist(err) {
			t.Fatalf("Couldn't find sample.wav: %v", err)
		}
	})

	var wavFormat *WAVFormat
	var err error

	t.Run("File Reading", func(t *testing.T) {
		wavFormat, err = NewWAVFormat(sampleWavPath)
		if err != nil {
			t.Fatalf("NewWAVFormat failed: %v", err)
		}
	})

	t.Run("Basic Properties", func(t *testing.T) {
		tests := []struct {
			name     string
			got      interface{}
			expected interface{}
		}{
			{"Sample Rate", wavFormat.Samplerate, expectedSampleRate},
			{"Channels", wavFormat.NumChannels, expectedChannels},
			{"Bit Depth", wavFormat.BitsPerSample, expectedBitDepth},
			{"ChunkID", string(wavFormat.ChunkID[:]), expectedChunk},
			{"Format", string(wavFormat.Format[:]), expectedFormat},
			{"Subchunk1ID", string(wavFormat.Subchunk1ID[:]), expectedSubChunkOne},
			{"Subchunk2ID", string(wavFormat.Subchunk2ID[:]), expectedsubChunkTwo},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.got != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, tt.got)
				}
			})
		}
	})

	t.Run("Additional Information", func(t *testing.T) {
		t.Logf("Sample Rate: %d", wavFormat.Samplerate)
		t.Logf("Channels: %d", wavFormat.NumChannels)
		t.Logf("Bit Depth: %d", wavFormat.BitsPerSample)
		t.Logf("Audio Format: %d", wavFormat.AudioFormat)
		t.Logf("Byte Rate: %d", wavFormat.ByteRate)
		t.Logf("Block Align: %d", wavFormat.BlockAlign)
		t.Logf("Data Size: %d bytes", wavFormat.Subchunk2Size)
	})
}
