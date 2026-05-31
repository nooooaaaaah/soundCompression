package flac

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"log"
	"math"
	"os"
	"sync"

	"github.com/nooooaaaaah/soundcompression/audio"
)

const (
	FlacMarker          = "fLaC"
	StreamInfoSize      = 34
	DefaultMinBlockSize = 4096
	DefaultMaxBlockSize = 4096
)

// The Encoder struct contains:
//   - input: an audio.Format interface representing the audio data to be encoded.
//   - output: a file where the encoded FLAC data will be written.
//   - minBlockSize and maxBlockSize: parameters that define the minimum and maximum block sizes for encoding.
//   - md5sum: a byte slice to store the MD5 checksum of the unencoded audio data.
//   - verbose: a boolean flag to enable verbose logging.
type Encoder struct {
	mu           sync.Mutex
	input        audio.Format
	output       *os.File
	minBlockSize int
	maxBlockSize int
	md5Hash      hash.Hash
	md5sum       []byte
	logging      bool
}

type bitWriter struct {
	buf      *bytes.Buffer
	buffer   byte
	bitCount int
}

func newBitWriter() *bitWriter {
	return &bitWriter{
		buf: &bytes.Buffer{},
	}
}

func (bw *bitWriter) writeBit(bit byte) {
	if bit&1 == 1 {
		bw.buffer |= 1 << (7 - bw.bitCount)
	}
	bw.bitCount++
	if bw.bitCount == 8 {
		bw.buf.WriteByte(bw.buffer)
		bw.buffer = 0
		bw.bitCount = 0
	}
}

func (bw *bitWriter) writeBits(val uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		bw.writeBit(byte((val >> uint(i)) & 1))
	}
}

func (bw *bitWriter) flush() {
	if bw.bitCount > 0 {
		bw.buf.WriteByte(bw.buffer)
		bw.buffer = 0
		bw.bitCount = 0
	}
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

	// Enforce the requirements of a FLAC encoder
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
	e.mu.Lock()
	defer e.mu.Unlock()
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

	// marker for FLAC metadata
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
 4. Packs sample rate (20 bits), channels (3 bits), bit depth (5 bits), and total samples (36 bits) into a single uint64.
 5. Writes the packed bitfield into the STREAMINFO array.
 7. Copies the MD5 checksum of the unencoded audio data into the array.
 8. Writes the STREAMINFO block to the output file.

This function is crucial because the STREAMINFO block provides the decoder with all the necessary parameters to correctly interpret the audio data. Without this information, the decoder would not know how to process the audio stream.
*/
func (e *Encoder) writeStreamInfo() error {
	if e.logging {
		log.Println("Writing STREAMINFO metadata block")
	}
	if e.minBlockSize < 16 || e.minBlockSize > 65535 {
		return fmt.Errorf("invalid min block size: %d (must be 16-65535)", e.minBlockSize)
	}

	// STREAMINFO block should be 34 bytes long and contain the following:
	// - Minimum block size (2 bytes)
	// - Maximum block size (2 bytes)
	// - Minimum frame size (3 bytes)
	// - Maximum frame size (3 bytes)
	// - Sample rate (20 bits), channels (3 bits), bit depth (5 bits), total samples (36 bits)
	//   packed into a single uint64 at bytes 10-17
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
func (e *Encoder) writeChannelSubframe(buf *bytes.Buffer, samples []int32, bitDepth int) error {
	if allEqual(samples) {
		return e.writeConstantSubframe(buf, samples[0], bitDepth)
	}

	// Try FIXED orders
	bestOrder := 0
	bestSize := e.estimateFixedSubframeSize(samples, 0)
	isLPC := false
	for order := 1; order <= 4; order++ {
		if size := e.estimateFixedSubframeSize(samples, order); size < bestSize {
			bestSize = size
			bestOrder = order
		}
	}

	// Try LPC orders for sufficiently large blocks
	if len(samples) >= 32 {
		lpcOrders := []int{8, 10, 12}
		for _, order := range lpcOrders {
			if order >= len(samples)/2 {
				continue
			}
			if size := e.estimateLPCSubframeSize(samples, order); size < bestSize {
				bestSize = size
				bestOrder = order
				isLPC = true
			}
		}
	}

	if isLPC {
		return e.writeLPCSubframe(buf, samples, bitDepth, bestOrder)
	}
	return e.writeFixedSubframe(buf, samples, bitDepth, bestOrder)
}

func (e *Encoder) encodeBlock(samples []int32, frameNumber int) error {
	blockSize := len(samples) / e.input.Channels()
	channels := e.input.Channels()
	bitDepth := e.input.BitDepth()

	// Try independent channels
	var indBuf bytes.Buffer
	for ch := range channels {
		chanSamples := make([]int32, blockSize)
		for i := range blockSize {
			chanSamples[i] = samples[i*channels+ch]
		}
		if err := e.writeChannelSubframe(&indBuf, chanSamples, bitDepth); err != nil {
			return err
		}
	}

	// Try mid-side for stereo
	var msBuf bytes.Buffer
	tryMidSide := channels == 2
	if tryMidSide {
		mid := make([]int32, blockSize)
		side := make([]int32, blockSize)
		for i := range blockSize {
			mid[i] = (samples[i*2] + samples[i*2+1]) >> 1
			side[i] = samples[i*2] - samples[i*2+1]
		}
		if err := e.writeChannelSubframe(&msBuf, mid, bitDepth); err != nil {
			return err
		}
		if err := e.writeChannelSubframe(&msBuf, side, bitDepth); err != nil {
			return err
		}
	}

	// Pick best channel assignment
	chanAssign := byte(channels - 1)
	subframeBytes := indBuf.Bytes()
	if tryMidSide && msBuf.Len() < indBuf.Len() {
		chanAssign = 10 // mid-side
		subframeBytes = msBuf.Bytes()
	}

	// Frame header
	header := []byte{
		0xFF, 0xF8,
		e.blockSizeCode()<<4 | e.sampleRateCode(),
		chanAssign<<4 | e.sampleSizeCode()<<1,
		byte(frameNumber),
	}
	header = append(header, crc8(header))

	var frameBuf bytes.Buffer
	if _, err := frameBuf.Write(header); err != nil {
		return err
	}
	if _, err := frameBuf.Write(subframeBytes); err != nil {
		return err
	}

	// CRC-16 footer
	crc := crc16(frameBuf.Bytes())
	crcBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(crcBytes, crc)
	if _, err := frameBuf.Write(crcBytes); err != nil {
		return err
	}

	_, err := e.output.Write(frameBuf.Bytes())
	return err
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
func (e *Encoder) predictSamples(samples []int32, order int) ([]int32, []int32) {
	if e.logging {
		log.Println("Predicting samples using LPC")
	}

	if len(samples) == 0 {
		return []int32{}, []int32{}
	}

	predicted := make([]int32, len(samples))
	residuals := make([]int32, len(samples))

	for i, s := range samples {
		var pred int32
		if i < order {
			pred = 0
		} else {
			switch order {
			case 0:
				pred = 0
			case 1:
				pred = samples[i-1]
			case 2:
				pred = 2*samples[i-1] - samples[i-2]
			case 3:
				pred = 3*samples[i-1] - 3*samples[i-2] + samples[i-3]
			case 4:
				pred = 4*samples[i-1] - 6*samples[i-2] + 4*samples[i-3] - samples[i-4]
			}
		}
		predicted[i] = pred
		residuals[i] = s - pred
	}

	return predicted, residuals
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
func (e *Encoder) encodeResidual(bw *bitWriter, residual []int32, k int) {
	if len(residual) == 0 {
		return
	}

	for _, r := range residual {
		// Map signed to unsigned (zigzag encoding)
		var u uint32
		if r < 0 {
			u = uint32((-r)<<1) - 1
		} else {
			u = uint32(r << 1)
		}

		// Rice encode: quotient in unary + remainder in binary
		q := u >> k
		rem := u & ((1 << k) - 1)

		// Write q ones followed by a zero (unary)
		for range q {
			bw.writeBit(1)
		}
		bw.writeBit(0)

		// Write remainder bits (k bits)
		if k > 0 {
			bw.writeBits(rem, k)
		}
	}
}

func calcRiceParameter(residual []int32) int {
	var sum int64
	for _, r := range residual {
		if r < 0 {
			sum += int64(-r)
		} else {
			sum += int64(r)
		}
	}
	if len(residual) == 0 {
		return 0
	}
	avg := sum / int64(len(residual))
	k := 0
	for k < 14 && (1<<k) < int(avg) {
		k++
	}
	return k
}

func (e *Encoder) writeFixedSubframe(buf *bytes.Buffer, samples []int32, bitDepth int, order int) error {
	// Validate order: fixed predictor order must be 0–4
	if order < 0 || order > 4 {
		return fmt.Errorf("invalid fixed predictor order: %d (must be 0–4)", order)
	}
	if len(samples) < order {
		return fmt.Errorf("not enough samples (%d) for fixed subframe order %d", len(samples), order)
	}

	// 1. byte: 0x20 | order (subframe header)
	if err := buf.WriteByte(0x20 | byte(order)); err != nil {
		return err
	}

	// 2. warm-up: order raw samples via sampleToBytes
	for i := 0; i < order && i < len(samples); i++ {
		if _, err := buf.Write(e.sampleToBytes(samples[i])); err != nil {
			return err
		}
	}

	// 3. compute residuals via predictSamples
	_, residuals := e.predictSamples(samples, order)

	// 4. byte: residual coding method (order << 2 | method) method = 0
	if err := buf.WriteByte(0x00); err != nil {
		return err
	}

	// 5. byte: Rice parameter (k << 4) — upper nibble, lower nibble (partition order) = 0
	res := residuals[order:]
	k := calcRiceParameter(res)
	if err := buf.WriteByte(byte(k) << 4); err != nil {
		return err
	}

	// 6. encode residual data using Rice coding
	bw := newBitWriter()
	e.encodeResidual(bw, res, k)
	bw.flush()
	if _, err := buf.Write(bw.buf.Bytes()); err != nil {
		return err
	}

	return nil
}

func (e *Encoder) writeConstantSubframe(buf *bytes.Buffer, sample int32, bitDepth int) error {
	if err := buf.WriteByte(0x00); err != nil {
		return err
	}
	_, err := buf.Write(e.sampleToBytes(sample))
	return err
}

func (e *Encoder) estimateFixedSubframeSize(samples []int32, order int) int {
	bytesPerSample := e.input.BitDepth() / 8
	size := 1 + order*bytesPerSample + 2 // header + warm-up + method + Rice param

	_, residuals := e.predictSamples(samples, order)
	res := residuals[order:]
	k := calcRiceParameter(res)

	bits := 0
	for _, r := range res {
		var u uint32
		if r < 0 {
			u = uint32((-r)<<1) - 1
		} else {
			u = uint32(r << 1)
		}
		q := u >> k
		bits += int(q) + 1 + k // unary + stop + remainder
	}
	return size + (bits+7)/8
}

func computeLPCCoeff(samples []int32, order int) ([]int32, int) {
	n := len(samples)

	// Autocorrelation
	R := make([]float64, order+1)
	for k := 0; k <= order; k++ {
		var sum float64
		for i := 0; i < n-k; i++ {
			sum += float64(samples[i]) * float64(samples[i+k])
		}
		R[k] = sum
	}

	// Levinson-Durbin recursion
	a := make([]float64, order+1)
	E := R[0]
	if E < 1e-10 {
		return nil, 0
	}

	for i := 1; i <= order; i++ {
		sum := R[i]
		for j := 1; j < i; j++ {
			sum -= a[j] * R[i-j]
		}
		ki := sum / E
		a[i] = ki
		for j := 1; j < i; j++ {
			a[j] -= ki * a[i-j]
		}
		E *= 1.0 - ki*ki
		if E < 1e-10 {
			break
		}
	}

	shift := 12
	maxAbs := 0.0
	for j := 1; j <= order; j++ {
		v := a[j]
		if v < 0 {
			v = -v
		}
		if v > maxAbs {
			maxAbs = v
		}
	}

	if maxAbs < 1e-10 {
		return nil, 0
	}

	shiftN := 1 << shift
	scale := float64(shiftN) / maxAbs
	if scale > float64(1<<20) {
		scale = float64(1 << 20)
	}

	coeff := make([]int32, order)
	for j := 1; j <= order; j++ {
		coeff[j-1] = int32(math.Round(a[j] * scale))
	}

	return coeff, shift
}

func lpcResidual(samples []int32, coeff []int32, shift int) []int32 {
	order := len(coeff)
	residuals := make([]int32, len(samples))
	for i := range samples {
		var pred int64
		for j := 0; j < order; j++ {
			if i > j {
				pred += int64(coeff[j]) * int64(samples[i-j-1])
			}
		}
		pred >>= shift
		residuals[i] = samples[i] - int32(pred)
	}
	return residuals
}

func (e *Encoder) estimateLPCSubframeSize(samples []int32, order int) int {
	coeff, shift := computeLPCCoeff(samples, order)
	if coeff == nil {
		return int(^uint(0) >> 1)
	}

	bytesPerSample := e.input.BitDepth() / 8
	size := 1 + order*bytesPerSample + 2 // header + warm-up + precision/shift + residual method
	size += len(coeff) * 2               // 2 bytes per coefficient

	residuals := lpcResidual(samples, coeff, shift)
	res := residuals[order:]
	k := calcRiceParameter(res)

	bits := 0
	for _, r := range res {
		var u uint32
		if r < 0 {
			u = uint32((-r)<<1) - 1
		} else {
			u = uint32(r << 1)
		}
		q := u >> k
		bits += int(q) + 1 + k
	}
	return size + (bits+7)/8
}

func (e *Encoder) writeLPCSubframe(buf *bytes.Buffer, samples []int32, bitDepth int, order int) error {
	coeff, shift := computeLPCCoeff(samples, order)
	if coeff == nil {
		return e.writeFixedSubframe(buf, samples, bitDepth, 0)
	}

	// Subframe header: LPC
	// bit 7 = 0, bits 6-5 = 01 (LPC marker), bits 4-1 = (order-1), bit 0 = 0 (no wasted bits)
	if err := buf.WriteByte(0x40 | byte((order-1)&0x0F)<<1); err != nil {
		return err
	}

	// Warm-up samples
	for i := 0; i < order && i < len(samples); i++ {
		if _, err := buf.Write(e.sampleToBytes(samples[i])); err != nil {
			return err
		}
	}

	// Coefficient precision byte: upper 4 bits = precision (bits per coeff), lower 4 bits = shift
	precision := uint32(12)
	if err := buf.WriteByte(byte(precision)<<4 | byte(shift)&0x0F); err != nil {
		return err
	}

	// Write coefficients (2 bytes each, little-endian, 12-bit signed)
	for _, c := range coeff {
		coefBytes := make([]byte, 2)
		binary.LittleEndian.PutUint16(coefBytes, uint16(int16(c)))
		if _, err := buf.Write(coefBytes); err != nil {
			return err
		}
	}

	// Residual coding (same as FIXED subframe)
	residuals := lpcResidual(samples, coeff, shift)
	res := residuals[order:]
	k := calcRiceParameter(res)
	if err := buf.WriteByte(0x00); err != nil { // residual method
		return err
	}
	if err := buf.WriteByte(byte(k) << 4); err != nil { // Rice parameter
		return err
	}

	bw := newBitWriter()
	e.encodeResidual(bw, res, k)
	bw.flush()
	if _, err := buf.Write(bw.buf.Bytes()); err != nil {
		return err
	}

	return nil
}

// Close closes the output FLAC file, ensuring all data is properly written and resources are released.
func (e *Encoder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.logging {
		log.Println("Closing output file")
	}

	var outputErr error
	if e.output != nil {
		outputErr = e.output.Close()
	}
	return outputErr
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

func allEqual(samples []int32) bool {
	if len(samples) == 0 {
		return false
	}
	first := samples[0]
	for _, s := range samples[1:] {
		if s != first {
			return false
		}
	}
	return true
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

func (e *Encoder) bestFixedOrder(samples []int32, bitDepth int) int {
	bytesPerSample := bitDepth / 8
	bestOrder := 0
	bestSize := int(^uint(0) >> 1) // max int

	for order := 0; order <= 4; order++ {
		// Fixed overhead: subframe header + warm-up + residual method/param bytes
		totalSize := 1 + order*bytesPerSample + 2

		// Residual size estimate
		_, residuals := e.predictSamples(samples, order)
		res := residuals[order:]
		k := calcRiceParameter(res)

		totalBits := 0
		for _, r := range res {
			var u uint32
			if r < 0 {
				u = uint32((-r)<<1) - 1
			} else {
				u = uint32(r << 1)
			}
			q := u >> k
			totalBits += int(q) + 1 + k // unary + stop + remainder
		}
		totalSize += (totalBits + 7) / 8 // round up to bytes

		if totalSize < bestSize {
			bestSize = totalSize
			bestOrder = order
		}
	}
	return bestOrder
}
