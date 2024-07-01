package flac

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/nooooaaaaah/soundcompression/audio"
)

const (
	FlacMarker          = "fLaC"
	StreamInfoSize      = 34
	DefaultMinBlockSize = 4096
	DefaultMaxBlockSize = 4096
)

type Encoder struct {
	input        audio.Format
	output       *os.File
	minBlockSize int
	maxBlockSize int
	md5sum       []byte
}

func NewEncoder(input audio.Format, outputPath string) (*Encoder, error) {
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("error creating output file: %w", err)
	}

	return &Encoder{
		input:        input,
		output:       outputFile,
		minBlockSize: DefaultMinBlockSize,
		maxBlockSize: DefaultMaxBlockSize,
	}, nil
}

func (e *Encoder) Encode() error {
	// Main encoding logic
	return nil
}

func (e *Encoder) writeStreamHeader() error {
	// marker for flac metadata
	_, err := e.output.Write([]byte("fLaC"))
	if err != nil {
		return err
	}

	// Write STREAMINFO metadata block
	err = e.writeStreamInfo()
	if err != nil {
		return err
	}

	// TODO more metadata blocks

	return nil
}

func (e *Encoder) writeStreamInfo() error {
	// ensures size of 34 bytes
	_, err := e.output.Write([]byte{0x00, 0x00, 0x00, 0x22})
	if err != nil {
		return err
	}

	// create block for stream info
	// array is 34 bytes cause thats size of
	// streaminfo
	streamInfo := make([]byte, 34)
	binary.BigEndian.PutUint16(streamInfo[0:2], uint16(e.minBlockSize))
	binary.BigEndian.PutUint16(streamInfo[2:4], uint16(e.maxBlockSize))
	// bytes 4-10 represent max and min frame size TODO
	// binary.BigEndian.PutUint16(streamInfo[10:14], uint16(e.sampleRate))

	// streamInfo[14] = byte(e.channels-1)<<4 | byte(e.bitDepth-1)

	// binary.BigEndian.PutUint32(streamInfo[18:23], uint32(e.totalSamples))

	// Write MD5 signature of the unencoded audio Data
	copy(streamInfo[18:], e.md5sum)

	_, err = e.output.Write(streamInfo)
	return err
}

func (e *Encoder) writeStreamFooter() error {
	return fmt.Errorf("not implemented")
}

func (e *Encoder) encodeBlock(samples []int32) error {
	// Implementation...
	return fmt.Errorf("not implemented")
}

func (e *Encoder) predictSamples(samples []int32) ([]int32, []int32) {
	// Implementation...
	return nil, nil
}

func (e *Encoder) encodeResidual(residual []int32) []byte {
	// Implementation...
	return nil
}

// Close closes the output flac file
func (e *Encoder) Close() error {
	var outputErr error
	if e.output != nil {
		outputErr = e.output.Close()
	}
	return outputErr
}
