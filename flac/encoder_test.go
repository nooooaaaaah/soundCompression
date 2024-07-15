package flac

import (
	"os"
	"testing"

	"github.com/nooooaaaaah/soundcompression/audio"
)

func TestNewEncoder(t *testing.T) {
	tests := []struct {
		name          string
		audioFilePath string
		outputPath    string
		expectedErr   bool
		expectedInput audio.Format
		expectedMinBS int
		expectedMaxBS int
	}{
		{
			name:          "Invalid audio file path",
			audioFilePath: "../nonexistent.wav",
			outputPath:    "test_output.flac",
			expectedErr:   true,
			expectedInput: nil,
			expectedMinBS: 0,
			expectedMaxBS: 0,
		},
		{
			name:          "Invalid output path",
			audioFilePath: "../sample.wav",
			outputPath:    "/invalid_path/test_output.flac",
			expectedErr:   true,
			expectedInput: nil,
			expectedMinBS: DefaultMinBlockSize,
			expectedMaxBS: DefaultMaxBlockSize,
		},
		{
			name:          "Invalid audio format",
			audioFilePath: "../sample.txt",
			outputPath:    "test_output.flac",
			expectedErr:   true,
			expectedInput: nil,
			expectedMinBS: DefaultMinBlockSize,
			expectedMaxBS: DefaultMaxBlockSize,
		}, {
			name:          "Valid WAV input",
			audioFilePath: "../sample.wav",
			outputPath:    "test_output.flac",
			expectedErr:   false,
			expectedInput: nil, // Will be set after creating the audioFormat
			expectedMinBS: DefaultMinBlockSize,
			expectedMaxBS: DefaultMaxBlockSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audioFormat, err := audio.NewWAVFormat(tt.audioFilePath)
			t.Logf("format: %v, error %v", audioFormat, err)
			if (err != nil) != tt.expectedErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
			}
			if err != nil {
				return
			}
			defer audioFormat.Close()

			tt.expectedInput = audioFormat

			encoder, err := NewEncoder(audioFormat, tt.outputPath, true)
			if (err != nil) != tt.expectedErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
			}
			if err != nil {
				return
			}
			defer os.Remove(tt.outputPath)
			defer encoder.Close()

			if encoder.input != tt.expectedInput {
				t.Errorf("expected input to be %v, got %v", tt.expectedInput, encoder.input)
			}
			if encoder.minBlockSize != tt.expectedMinBS {
				t.Errorf("expected minBlockSize to be %d, got %d", tt.expectedMinBS, encoder.minBlockSize)
			}
			if encoder.maxBlockSize != tt.expectedMaxBS {
				t.Errorf("expected maxBlockSize to be %d, got %d", tt.expectedMaxBS, encoder.maxBlockSize)
			}
		})
	}
}
func TestWriteStreamInfo(t *testing.T) {
	tests := []struct {
		name         string
		minBlockSize int
		maxBlockSize int
		sampleRate   int
		channels     int
		bitDepth     int
		totalSamples int64
		md5sum       []byte
		expectedErr  bool
	}{
		{
			name:         "Valid STREAMINFO",
			minBlockSize: 4096,
			maxBlockSize: 4096,
			sampleRate:   44100,
			channels:     2,
			bitDepth:     16,
			totalSamples: 44100,
			md5sum:       make([]byte, 16),
			expectedErr:  false,
		},
		{
			name:         "Invalid minBlockSize",
			minBlockSize: 0,
			maxBlockSize: 4096,
			sampleRate:   44100,
			channels:     2,
			bitDepth:     16,
			totalSamples: 44100,
			md5sum:       make([]byte, 16),
			expectedErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audioFormat, err := audio.NewWAVFormat("../sample.wav")
			if err != nil {
				t.Fatalf("failed to create audio format: %v", err)
			}
			defer audioFormat.Close()

			outputFile, err := os.CreateTemp("", "test_output_*.flac")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(outputFile.Name())

			encoder := &Encoder{
				output:       outputFile,
				input:        audioFormat,
				minBlockSize: tt.minBlockSize,
				maxBlockSize: tt.maxBlockSize,
				md5sum:       tt.md5sum,
				logging:      false,
			}

			err = encoder.writeStreamInfo()
			if (err != nil) != tt.expectedErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
			}
		})
	}
}
