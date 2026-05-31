package flac

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nooooaaaaah/soundcompression/audio"
)

func TestNewEncoder(t *testing.T) {
	tests := []struct {
		name          string
		audioFilePath string
		outputPath    string
		expectedErr   bool
		expectedMinBS int
		expectedMaxBS int
	}{
		{
			name:          "Invalid audio file path",
			audioFilePath: "../nonexistent.wav",
			outputPath:    "test_output.flac",
			expectedErr:   true,
			expectedMinBS: 0,
			expectedMaxBS: 0,
		},
		{
			name:          "Invalid audio format",
			audioFilePath: "../testdata/bad.wav",
			outputPath:    "test_output.flac",
			expectedErr:   true,
			expectedMinBS: DefaultMinBlockSize,
			expectedMaxBS: DefaultMaxBlockSize,
		}, {
			name:          "Valid WAV input",
			audioFilePath: "../testdata/sample.wav",
			outputPath:    "test_output.flac",
			expectedErr:   false,
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

			outputPath := filepath.Join(t.TempDir(), tt.outputPath)
			encoder, err := NewEncoder(audioFormat, outputPath, true)
			if (err != nil) != tt.expectedErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
			}
			if err != nil {
				return
			}
			defer encoder.Close()

			if encoder.input != audioFormat {
				t.Errorf("expected input to be %v, got %v", audioFormat, encoder.input)
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
			audioFormat, err := audio.NewWAVFormat("../testdata/sample.wav")
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

func TestEncoderEncode(t *testing.T) {
	audioFormat, err := audio.NewWAVFormat("../testdata/sample.wav")
	if err != nil {
		t.Fatalf("failed to create audio format: %v", err)
	}
	defer audioFormat.Close()

	outputPath := filepath.Join(t.TempDir(), "output.flac")
	encoder, err := NewEncoder(audioFormat, outputPath, false)
	if err != nil {
		t.Fatalf("NewEncoder failed: %v", err)
	}

	if err := encoder.Encode(); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	encoder.Close()

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output file stat failed: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if string(data[:4]) != "fLaC" {
		t.Errorf("output starts with %q, want %q", data[:4], "fLaC")
	}
}

func Test_crc16(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint16
	}{
		{"empty", []byte{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crc16(tt.data)
			if got != tt.want {
				t.Errorf("crc16() = %04x, want %04x", got, tt.want)
			}
		})
	}
}

func TestEncoder_writeFixedSubframe(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		input      audio.Format
		outputPath string
		logging    bool
		// Named input parameters for target function.
		buf      *bytes.Buffer
		samples  []int32
		bitDepth int
		order    int
		wantErr  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := NewEncoder(tt.input, tt.outputPath, tt.logging)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}
			gotErr := e.writeFixedSubframe(tt.buf, tt.samples, tt.bitDepth, tt.order)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("writeFixedSubframe() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("writeFixedSubframe() succeeded unexpectedly")
			}
		})
	}
}
