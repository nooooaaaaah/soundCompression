# Notes

## TODO

- [x] Complete the STREAMINFO metadata block

- [x] Update the writeStreamInfo method to include all required fields:
  - [x] Sample rate: Use binary.BigEndian.PutUint32() to write the 20-bit sample rate
  - [x] Channels: Write the number of channels minus 1 (3 bits)
  - [x] Bits per sample: Write the bits per sample minus 1 (5 bits)
  - [x] Total samples: Use binary.BigEndian.PutUint64() to write the 36-bit total samples count
  - [x] MD5 signature: Calculate and write the 16-byte MD5 sum of the unencoded audio data

- [ ] Implement the main Encode method
  - [ ] Read input audio data in chunks (use the block size as the chunk size)
  - [ ] For each chunk, call encodeBlock
  - [ ] Write the encoded blocks to the output file
  - [ ] Keep track of the total samples encoded

- [ ] Implement the encodeBlock method
  - [ ] Implement subframe encoding for each channel
  - [ ] Choose the best subframe type (CONSTANT, VERBATIM, FIXED, or LPC)
  - [ ] Encode the subframe
  - [ ] Implement interchannel decorrelation if needed
  - [ ] Write the frame header, encoded subframes, and frame footer

- [ ] Implement predictSamples method
  - [ ] Implement fixed prediction (orders 0-4)
  - [ ] Implement LPC prediction (use the Levinson-Durbin algorithm for coefficient calculation)
  - [ ] Return both the predicted samples and the residuals

- [ ] Implement encodeResidual method
  - [ ] Implement Rice coding for the residuals
  - [ ] Choose the best Rice parameter
  - [ ] Encode the residuals using the chosen Rice parameter

- [ ] Implement frame header and footer writing
  - [ ] Write the sync code, blocking strategy, block size, sample rate, channel assignment, sample size, and frame number
  - [ ] Calculate and write the CRC-16 for the footer

- [ ] Implement MD5 calculation
  - [ ] Use the crypto/md5 package to calculate the MD5 sum of the unencoded audio data
  - [ ] Store this in the Encoder struct for use in the STREAMINFO block

- [ ] Implement writeStreamFooter method
  - [ ] Write the required markers to indicate the end of the FLAC stream

- [ ] Add error handling and resource management
  - [ ] Add appropriate error checks throughout the code
  - [ ] Ensure all resources (especially file handles) are properly closed in case of errors

- [ ] Implement additional metadata blocks (optional)
  - [ ] Implement methods to write SEEKTABLE, VORBIS_COMMENT, or other metadata blocks
  - [ ] Call these methods in writeStreamHeader after writing STREAMINFO

- [ ] Optimize encoding parameters
  - [ ] Implement logic to choose optimal block sizes
  - [ ] Experiment with different LPC orders to find the best trade-off between compression and speed

- [ ] Add progress reporting and statistics
  - [ ] Implement methods to track and report encoding progress
  - [ ] Calculate and report compression ratio

- [ ] Implement multi-threaded encoding (optional)
  - [ ] Use Go's concurrency features to encode multiple blocks in parallel
  - [ ] Ensure thread-safe writing to the output file

- [ ] Add input validation and error checking
  - [ ] Validate input audio format (sample rate, bit depth, etc.)
  - [ ] Check for unsupported or invalid configurations

- [ ] Implement a basic command-line interface
  - [ ] Use the flag package to parse command-line arguments
  - [ ] Allow users to specify input file, output file, and encoding options

- [ ] More metadata blocks
- [ ] Max and min block/frame sizes should be better

## Unsure About
