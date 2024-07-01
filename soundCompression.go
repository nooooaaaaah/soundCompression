package main

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
)

const WAVHeaderSize = 44

type FlacEncoder struct {
	inputFile    *os.File
	outputFile   *os.File
	sampleRate   int
	channels     int
	bitDepth     int
	totalSamples uint64
	minBlockSize int
	maxBlockSize int
	md5sum       []byte
}

// Wav header should be 44 bytes in size
// We retun the full header and the info
// is extracted as it's needed
func readWAVHeader(file *os.File) ([]byte, error) {
	header := make([]byte, WAVHeaderSize)
	_, err := file.Read(header)
	if err != nil {
		return nil, fmt.Errorf("error reading WAV header: %w", err)
	}
	return header, nil
}

// WAV only
func getSampleRate(header []byte) (int, error) {
	if len(header) < WAVHeaderSize {
		return 0, fmt.Errorf("header is too short")
	}
	return int(binary.LittleEndian.Uint32(header[24:28])), nil
}

// WAV only
func getNumChannels(header []byte) (int, error) {
	if len(header) < WAVHeaderSize {
		return 0, fmt.Errorf("header is too short")
	}
	return int(binary.LittleEndian.Uint16(header[22:24])), nil
}

// WAV only
func getBitDepth(header []byte) (int, error) {
	if len(header) < WAVHeaderSize {
		return 0, fmt.Errorf("header is too short")
	}
	return int(binary.LittleEndian.Uint16(header[34:36])), nil
}

// WAV only
func getTotalSamples(header []byte, file *os.File) (uint64, error) {
	// Get the size of the file
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}

	dataSize := fileInfo.Size() - int64(WAVHeaderSize)

	bitDepth := int64(binary.LittleEndian.Uint16(header[34:36]))
	bytesPerSample := bitDepth / 8

	numChannels := int64(binary.LittleEndian.Uint16(header[22:24]))

	totalSamples := uint64(dataSize / (bytesPerSample * numChannels))
	return totalSamples, nil
}

// the input and output file are left open
// defer or handle the close at the end
// only takes wav at the moment
func NewFlacEncoder(inputPath, outputPath string) (*FlacEncoder, error) {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}

	outputFile, err := os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("error handling output file: %w", err)
	}

	header, err := readWAVHeader(inputFile)
	if err != nil {
		return nil, fmt.Errorf("error reading wav header: %w", err)
	}

	sampleRate, err := getSampleRate(header)
	if err != nil {
		return nil, fmt.Errorf("Issue getting the sample rate: %w", err)
	}

	channels, err := getNumChannels(header)
	if err != nil {
		return nil, fmt.Errorf("error getting number of channels: %w", err)
	}

	bitDepth, err := getBitDepth(header)
	if err != nil {
		return nil, fmt.Errorf("error getting bit depth: %w", err)
	}

	totalSamples, err := getTotalSamples(header, inputFile)
	if err != nil {
		return nil, fmt.Errorf("error calculating total samples: %w", err)
	}

	// make configurable at some point
	minBlockSize := 4096
	maxBlockSize := 4096

	// Return initialized FlacEncoder
	return &FlacEncoder{
		inputFile:    inputFile,
		outputFile:   outputFile,
		sampleRate:   sampleRate,
		channels:     channels,
		bitDepth:     bitDepth,
		totalSamples: totalSamples,
		minBlockSize: minBlockSize,
		maxBlockSize: maxBlockSize,
	}, nil
}

func (f *FlacEncoder) Encode() error {
	// Main encoding logic
	_, err := f.inputFile.Seek(WAVHeaderSize, io.SeekStart)
	if err != nil {
		return fmt.Errorf("error seeking past WAV header: %w", err)
	}
	hasher := md5.New()
	buffer := make([]byte, 4096)
	for {
		n, err := f.inputFile.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading audio data: %w", err)
		}

		hasher.Write(buffer[:n])

		// TODO process and encode the audio data
		// Reads audio data from the input file, processes it, and encodes it into FLAC format.
	}
	f.md5sum = hasher.Sum(nil)

	// Write the FLAC stream header (including STREAMINFO)
	err = f.writeFlacStreamHeader()
	if err != nil {
		return fmt.Errorf("error writing FLAC stream header: %w", err)
	}

	return nil
}

func (f *FlacEncoder) readSamples(buffer []int32) (int, error) {
	return 0, nil
}

func (f *FlacEncoder) writeFlacStreamHeader() error {
	// marker for flac metadata
	_, err := f.outputFile.Write([]byte("fLaC"))
	if err != nil {
		return err
	}

	// Write STREAMINFO metadata block
	err = f.writeStreamInfo()
	if err != nil {
		return err
	}

	// TODO more metadata blocks

	return nil
}

// manditory metadata block
// precedes all other blocks
func (f *FlacEncoder) writeStreamInfo() error {
	// ensures size of 34 bytes
	_, err := f.outputFile.Write([]byte{0x00, 0x00, 0x00, 0x22})
	if err != nil {
		return err
	}

	// create block for stream info
	// array is 34 bytes cause thats size of
	// streaminfo
	streamInfo := make([]byte, 34)
	binary.BigEndian.PutUint16(streamInfo[0:2], uint16(f.minBlockSize))
	binary.BigEndian.PutUint16(streamInfo[2:4], uint16(f.maxBlockSize))
	// bytes 4-10 represent max and min frame size TODO
	binary.BigEndian.PutUint16(streamInfo[10:14], uint16(f.sampleRate))

	streamInfo[14] = byte(f.channels-1)<<4 | byte(f.bitDepth-1)

	binary.BigEndian.PutUint32(streamInfo[18:23], uint32(f.totalSamples))

	// Write MD5 signature of the unencoded audio Data
	copy(streamInfo[18:], f.md5sum)

	_, err = f.outputFile.Write(streamInfo)
	return err
}

func (f *FlacEncoder) writeStreamFooter() error {
	// Implement FLAC stream footer writing
	// This is a placeholder and needs to be implemented properly
	return nil
}

func (f *FlacEncoder) encodeBlock(samples []int32) error {
	// Encode a single block of audio
	return nil
}

func (f *FlacEncoder) predictSamples(samples []int32) ([]int32, []int32) {
	// Perform linear prediction
	// Return coefficients and residual
	return nil, nil
}

func (f *FlacEncoder) encodeResidual(residual []int32) []byte {
	// Encode residual using Rice coding or similar
	return nil
}

func countLines(filename string) (int, error) {
	cmd := exec.Command("wc", filename)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var count int
	_, err = fmt.Sscanf(string(output), "%d", &count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
