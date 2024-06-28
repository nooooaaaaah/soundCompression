package main

import (
	"os"
	"testing"
)

const sampleWavPath = "sample.wav"

// TestNewFlacEncoder tests the creation of a new FlacEncoder
func TestNewFlacEncoder(t *testing.T) {
	// Check if sample.wav exists
	if _, err := os.Stat(sampleWavPath); os.IsNotExist(err) {
		t.Fatalf("sample.wav does not exist. Please ensure it's in the correct location.")
	}

	// Test NewFlacEncoder
	encoder, err := NewFlacEncoder(sampleWavPath, "test_output.flac")
	if err != nil {
		t.Fatalf("NewFlacEncoder failed: %v", err)
	}

	// Check if the encoder was created with non-zero values
	// Note: We can't predict exact values without knowing the content of sample.wav
	if encoder.sampleRate != 44100 {
		t.Errorf("Expected non-zero sample rate, got 0")
	}
	if encoder.channels != 2 {
		t.Errorf("Expected non-zero number of channels, got 0")
	}
	if encoder.bitDepth != 16 {
		t.Errorf("Expected non-zero bit depth, got 0")
	}

	// Print the actual values for manual verification
	t.Logf("Sample Rate: %d", encoder.sampleRate)
	t.Logf("Channels: %d", encoder.channels)
	t.Logf("Bit Depth: %d", encoder.bitDepth)
}
