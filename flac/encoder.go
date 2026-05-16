package flac

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"math"
	"os"

	"log"

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
	md5Hash      hash.Hash
	md5sum       []byte
	logging      bool
}

// NewEncoder initializes a new Encoder instance for encoding audio data into the FLAC format.
// It takes an audio input format and an output file path as parameters.
// Returns a pointer to the Encoder instance and an error if any occurs during file creation.
func NewEncoder(input audio.Format, outputPath string, logging bool) (*Encoder, error) {
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("error creating output file: %w", err)
	}

	sampleRate := input.SampleRate()
	bitDepth := input.BitDepth()
	channels := input.Channels()

	if sampleRate < 1 || sampleRate > 1048575 {
		outputFile.Close()
		return nil, fmt.Errorf("sample rate must be between 1 and 1048575")
	}
	if bitDepth < 4 || bitDepth > 32 {
		outputFile.Close()
		return nil, fmt.Errorf("bit depth must be between 4 and 32")
	}
	if channels < 1 || channels > 8 {
		outputFile.Close()
		return nil, fmt.Errorf("channels must be between 1 and 8")
	}

	// Enforce the requriemnts of a flac encoder
	return &Encoder{
		input:        input,
		output:       outputFile,
		minBlockSize: DefaultMinBlockSize,
		maxBlockSize: DefaultMaxBlockSize,
		md5Hash:      md5.New(),
		logging:      logging,
	}, nil
}

/*
Encoder is responsible for encoding raw audio data into the FLAC format.

The Encoder struct contains:
  - input: an audio.Format interface representing the audio data to be encoded.
  - output: a file where the encoded FLAC data will be written.
  - minBlockSize and maxBlockSize: parameters that define the minimum and maximum block sizes for encoding.
  - md5sum: a byte slice to store the MD5 checksum of the unencoded audio data.
  - verbose: a boolean flag to enable verbose logging.

The Encode method is the main function that handles the encoding process. It performs the following steps:
 1. Writes the stream header, including the FLAC marker and STREAMINFO metadata block.
 2. Creates a buffer to hold audio samples.
 3. Reads audio samples from the input in blocks and encodes each block.
 4. Writes the stream footer to finalize the FLAC file.

Usage:
 1. Create an Encoder instance using NewEncoder by providing the audio input format and output file path.
 2. Call the Encode method to start the encoding process.
 3. Close the Encoder to ensure the output file is properly closed.
*/
func (e *Encoder) Encode() error {
	success := false
	defer func() {
		if !success {
			e.output.Close()
			os.Remove(e.output.Name())
		}
	}()
	if e.logging {
		log.Println("Starting encoding process")
	}

	// Write the stream header
	err := e.writeStreamHeader()
	if err != nil {
		return fmt.Errorf("error writing stream header: %w", err)
	}

	// Create a buffer to hold audio samples
	buffer := make([]int32, e.minBlockSize*e.input.Channels())
	frameNumber := 0
	for {
		// Read samples from the input
		n, err := e.input.ReadSamples(buffer)
		if err == io.EOF {
			break // End of file reached
		}
		if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}

		// Feed raw bytes into MD5
		for _, sample := range buffer[:n] {
			e.md5Hash.Write(e.sampleToBytes(sample))
		}

		// Encode the block of samples
		err = e.encodeBlock(buffer[:n], frameNumber)
		if err != nil {
			return fmt.Errorf("error encoding block: %w", err)
		}

		if e.logging {
			log.Printf("Encoded block of %d samples", n)
		}
		frameNumber++
	}

	e.md5sum = e.md5Hash.Sum(nil)

	if _, err := e.output.Seek(26, io.SeekStart); err != nil {
		return fmt.Errorf("error seeking to MD5 position: %w", err)
	}
	if _, err := e.output.Write(e.md5sum); err != nil {
		return fmt.Errorf("error writing MD5 to STREAMINFO: %w", err)
	}
	if _, err := e.output.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("error seeking to end for footer: %w", err)
	}
	// Write the stream footer
	err = e.writeStreamFooter()
	if err != nil {
		return fmt.Errorf("error writing stream footer: %w", err)
	}

	if e.logging {
		log.Println("Finished encoding process")
	}
	success = true
	return nil
}

/*
sampleToBytes converts a single int32 audio sample into its raw byte representation
based on the encoder's bit depth setting.

The function performs the following steps:
 1. Allocates a byte buffer sized to the number of bytes required for the bit depth.
 2. Converts the sample into the appropriate byte format based on the bit depth:
    - 8-bit: applies an unsigned bias of 128 to shift the signed value into unsigned range.
    - 16-bit: encodes the sample as a little-endian 16-bit integer.
    - 24-bit: encodes the sample as a little-endian 24-bit integer across 3 bytes.
    - 32-bit: encodes the sample as a little-endian 32-bit integer.
 3. Returns the resulting byte slice.

This function is used during the MD5 checksum calculation to feed raw audio bytes
into the hash, ensuring the checksum matches the unencoded audio data.
*/
func (e *Encoder) sampleToBytes(sample int32) []byte {
	buf := make([]byte, e.input.BitDepth()/8)
	switch e.input.BitDepth() {
	case 8:
		buf[0] = byte(sample + 128) // unsigned bias
	case 16:
		binary.LittleEndian.PutUint16(buf, uint16(int16(sample)))
	case 24:
		val := uint32(sample)
		buf[0] = byte(val)
		buf[1] = byte(val >> 8)
		buf[2] = byte(val >> 16)
	case 32:
		binary.LittleEndian.PutUint32(buf, uint32(sample))
	}
	return buf
}

/*
writeStreamHeader writes the initial FLAC stream header, which includes the FLAC marker and the STREAMINFO metadata block. This header is essential for any FLAC file as it signals the beginning of the FLAC stream and provides the decoder with necessary information about the audio data.

The function performs the following steps:

 1. Writes the FLAC marker "fLaC" to the output file, which is a mandatory identifier for FLAC streams.
 2. Calls writeStreamInfo to write the STREAMINFO metadata block, which contains crucial information about the audio stream, such as block sizes, sample rate, and MD5 checksum.

If any error occurs during these steps, the function returns the error to ensure proper error handling.
*/
func (e *Encoder) writeStreamHeader() error {
	if e.logging {
		log.Println("Writing stream header")
	}

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

/*
The writeStreamInfo function is responsible for writing the STREAMINFO metadata block, which is a mandatory block in the FLAC format. This block contains essential information about the audio stream, such as block sizes, sample rate, and MD5 checksum. The function performs the following steps:

 1. Writes the metadata block header indicating a STREAMINFO block with a size of 34 bytes.
 2. Creates a 34-byte array to store STREAMINFO data.
 3. Fills the array with the minimum and maximum block sizes.
 4. Writes the sample rate, left-shifted by 4 bits for alignment.
 5. Encodes the number of channels and bits per sample into a single byte.
 6. Writes the total number of samples.
 7. Copies the MD5 checksum of the unencoded audio data into the array.
 8. Writes the STREAMINFO block to the output file.

This function is crucial because the STREAMINFO block provides the decoder with all the necessary parameters to correctly interpret the audio data. Without this information, the decoder would not know how to process the audio stream.
*/
func (e *Encoder) writeStreamInfo() error {
	if e.logging {
		log.Println("Writing STREAMINFO metadata block")
	}

	// STREAMINFO block should be 34 bytes long and contain the following:
	// - Minimum block size (2 bytes)
	// - Maximum block size (2 bytes)
	// - Minimum frame size (3 bytes)
	// - Maximum frame size (3 bytes)
	// - Sample rate (20 bits, left-shifted by 4 bits for alignment)
	// - Number of channels (3 bits) and bits per sample (5 bits)
	// - Total number of samples (36 bits)
	// - MD5 signature of the unencoded audio data (16 bytes)

	// Write the metadata block header for STREAMINFO with size 34 bytes
	_, err := e.output.Write([]byte{0x80, 0x00, 0x00, 0x22})
	if err != nil {
		return err
	}

	// Create a byte array for STREAMINFO block, which is 34 bytes long
	streamInfo := make([]byte, 34)

	// Pack the minimum and maximum block sizes into bytes 0-3 of the streamInfo array
	binary.BigEndian.PutUint16(streamInfo[0:2], uint16(e.minBlockSize))
	binary.BigEndian.PutUint16(streamInfo[2:4], uint16(e.maxBlockSize))
	frameSize := uint32(6 + e.input.Channels()*(1+e.maxBlockSize*2) + 2)
	streamInfo[4] = byte(frameSize >> 16)
	streamInfo[5] = byte(frameSize >> 8)
	streamInfo[6] = byte(frameSize)
	streamInfo[7] = byte(frameSize >> 16)
	streamInfo[8] = byte(frameSize >> 8)
	streamInfo[9] = byte(frameSize)

	// Pack the sample rate (20 bits), channels (3 bits), bit depth (5 bits), and
	// total samples (36 bits) into a single uint64 by bit-shifting each field into
	// its correct position, then OR them together. The result is written into bytes
	// 10-18 of the streamInfo array in big-endian order.
	//
	// Bit layout of the packed uint64 (64 bits total):
	//   [63:44] - Sample rate (20 bits), shifted left by 44
	//   [43:41] - Channels minus 1 (3 bits), shifted left by 41
	//   [40:36] - Bit depth minus 1 (5 bits), shifted left by 36
	//   [35:0]  - Total number of samples (36 bits)
	packed := uint64(e.input.SampleRate())<<44 |
		uint64(e.input.Channels()-1)<<41 |
		uint64(e.input.BitDepth()-1)<<36 |
		(uint64(e.input.TotalSamples()) & 0xFFFFFFFFF)
	binary.BigEndian.PutUint64(streamInfo[10:18], packed)

	// Copy the 16-byte MD5 signature of the unencoded audio data into bytes 18-34
	copy(streamInfo[18:], e.md5sum)

	// Write the STREAMINFO block to the output
	_, err = e.output.Write(streamInfo)
	return err
}

// File ends after last frame no footer
// will always be true
func (e *Encoder) writeStreamFooter() error {
	if e.logging {
		log.Println("Writing stream footer")
	}
	return nil
}

/*
encodeBlock is responsible for encoding a block of audio samples. Currently, this function simply writes the raw PCM data to the output file in little-endian format. This is a placeholder implementation and does not perform actual FLAC encoding.

The purpose of this function is to provide a starting point for the encoding process. In a complete implementation, this function would handle the compression and encoding of audio samples according to the FLAC specification. For now, it allows the rest of the encoding pipeline to be tested with raw audio data.
*/
func (e *Encoder) encodeBlock(samples []int32, frameNumber int) error {
	blockSize := len(samples) / e.input.Channels()

	// --- Frame Header ---
	header := []byte{
		0xFF, // sync
		0xF8, // sync | reserved | blocking_strategy(fixed=0)
		e.blockSizeCode()<<4 | e.sampleRateCode(),
		e.channelAssignCode()<<4 | e.sampleSizeCode()<<1,
		byte(frameNumber), // frame number (simplified: single-byte for small values)
	}
	header = append(header, crc8(header))

	if _, err := e.output.Write(header); err != nil {
		return err
	}

	// --- Subframes (one per channel) ---
	channels := e.input.Channels()
	for ch := 0; ch < channels; ch++ {
		// Subframe header: VERBATIM (0x02)
		if _, err := e.output.Write([]byte{0x02}); err != nil {
			return err
		}

		// Raw samples for this channel
		buf := make([]byte, blockSize*2) // 2 bytes per 16-bit sample
		for i := 0; i < blockSize; i++ {
			sample := samples[i*channels+ch]
			binary.LittleEndian.PutUint16(buf[i*2:], uint16(int16(sample)))
		}
		if _, err := e.output.Write(buf); err != nil {
			return err
		}
	}

	// --- Frame Footer: CRC-16 ---
	// For now, placeholder — CRC-16 requires buffering subframe data,
	// or we compute it as we write. Simplest: write 0x0000.
	if _, err := e.output.Write([]byte{0x00, 0x00}); err != nil {
		return err
	}

	return nil
}

/*
predictSamples is intended to perform linear predictive coding (LPC) on the input samples. LPC is a tool used in audio signal processing to represent the spectral envelope of a digital signal of speech in compressed form, using the information of a linear predictive model.

The function should perform the following steps:

 1. Calculate the LPC coefficients for the given block of samples. These coefficients represent the filter that can predict the next sample based on previous samples.
 2. Generate the predicted samples using the LPC coefficients. This involves applying the filter to the previous samples to predict the next sample.
 3. Calculate the residuals, which are the differences between the actual samples and the predicted samples. These residuals are what will be encoded in the FLAC stream.

The function returns two slices of int32:
 1. The predicted samples, which are used to reconstruct the original signal during decoding.
 2. The residuals, which are the differences between the actual and predicted samples and will be encoded.

This function is crucial for the compression efficiency of the FLAC encoder, as it reduces the amount of data that needs to be stored by leveraging the predictability of audio signals.
*/
func (e *Encoder) predictSamples(samples []int32) ([]int32, []int32) {
	if e.logging {
		log.Println("Predicting samples using LPC")
	}

	// Implementation...
	return nil, nil
}

/*
encodeResidual encodes the residuals (differences between actual samples and predicted samples) into a compressed format suitable for FLAC.

Residual encoding significantly reduces the amount of data that needs to be stored. The residuals are typically encoded using Rice coding, a form of entropy coding efficient for this type of data.

The function performs the following steps:
 1. Determines the optimal Rice parameter for the residuals, balancing the trade-off between the size of the encoded data and the complexity of encoding.
 2. Encodes the residuals using the calculated Rice parameter by splitting them into groups and encoding each group with the Rice parameter.
 3. Returns the encoded residuals as a byte slice, which will be written to the FLAC stream.

Proper implementation of this function is crucial for achieving high compression ratios in the FLAC format.
*/
func (e *Encoder) encodeResidual(residual []int32) []byte {
	if e.logging {
		log.Println("Encoding residuals")
	}

	// Implementation...
	return nil
}

// Close closes the output flac file, ensuring all data is properly written and resources are released.
func (e *Encoder) Close() error {
	if e.logging {
		log.Println("Closing output file")
	}

	var outputErr error
	if e.output != nil {
		outputErr = e.output.Close()
	}
	return outputErr
}

/*
calcMinBlockSize calculates the minimum block size for FLAC encoding.

The function ensures the bit depth is within the valid range (4-32 bits) and that the number of channels is valid (1-8 channels). It performs the following steps:
 1. Validates the bit depth and channel count.
 2. Calculates the total number of bits required for the minimum block size (16 samples).
 3. Returns the minimum block size in bytes.

The minimum block size affects the overall compression efficiency and latency of the encoded audio. If the bit depth or channels are out of the valid range, the function returns 0 to indicate an error.
*/
func calcMinBlockSize(bitDepth, channels int) int {
	const minSamples = 16
	if bitDepth >= 4 && bitDepth <= 32 {
		if channels >= 1 && channels <= 8 {
			totalBits := minSamples * bitDepth * channels
			return int(math.Ceil(float64(totalBits) / 8))
		}
	}
	// Returning 0 indicates that the provided bit depth or channel count is invalid.
	return 0
}

func crc8(data []byte) byte {
	crc := byte(0)
	for _, b := range data {
		crc ^= b
		for range 8 {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ 0x07
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func crc16(data []byte) uint16 {
	crc := uint16(0)
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0x8005
			} else {
				crc >>= 1
			}
		}
	}
	return crc // no final xor
}

func (e *Encoder) blockSizeCode() byte {
	// 4096 = 2^12 → code 1101 (13)
	switch e.maxBlockSize {
	case 4096:
		return 13 // 1101
	// add others as needed
	default:
		return 13
	}
}

func (e *Encoder) sampleRateCode() byte {
	switch e.input.SampleRate() {
	case 44100:
		return 9 // 1001
	default:
		return 0 // get from STREAMINFO
	}
}

func (e *Encoder) channelAssignCode() byte {
	return byte(e.input.Channels() - 1) // 0-7, 0=mono, 1=stereo, etc.
}

func (e *Encoder) sampleSizeCode() byte {
	switch e.input.BitDepth() {
	case 8:
		return 1
	case 16:
		return 3
	case 24:
		return 5
	case 32:
		return 6
	default:
		return 0
	}
}
