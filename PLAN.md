# SoundCompression Project Plan

A learning project for FLAC encoding, audio streaming, and a TUI player written in Go.

## Current Status

The project has a WAV parser (`audio/`) and a partially implemented FLAC encoder (`flac/`).
The encoder writes a valid "fLaC" marker and STREAMINFO metadata block header, but:

- STREAMINFO bitfield packing is **bugged** (offsets and bit layout are wrong)
- MD5 checksum is never computed
- Frame encoding is a raw PCM passthrough (no FLAC frame structure)
- No prediction or residual coding
- Stream footer always returns "not implemented"
- WAV parser assumes sequential chunks (breaks on files with JUNK/LIST chunks)

## Phase 1: Bug Fixes (Current)

Fix all bugs to produce a valid FLAC bitstream.

### ~~P1.1 STREAMINFO bitfield~~
`flac/encoder.go:189-206` — Fix the packed bitfield at bytes 10-17 of STREAMINFO:
- 20-bit sample rate
- 3-bit (channels - 1)
- 5-bit (bits per sample - 1)
- 36-bit total samples
Write as a single uint64 big-endian. MD5 goes at bytes 18-33.

### ~~P1.2 Metadata block header~~
`flac/encoder.go:181` — Set `is_last=1` (byte `0x80`) since only STREAMINFO is written.

### ~~P1.3 MD5 hashing~~
`flac/encoder.go:27,205` + `flac/encoder.go:72-117` — `md5sum` is never populated, so the `copy` at line 205 copies 0 bytes. Hash raw audio during Encode loop using `crypto/md5`.

### ~~P1.4 Frame structure~~
`flac/encoder.go:224-237` — Implement FLAC frame header (sync code `0xFF 0xF8`, block size, sample rate, channel assignment, frame number), VERBATIM subframe, and frame footer (CRC-16).

### ~~P1.5 Stream footer~~
`flac/encoder.go:212-217` — Write end-of-stream marker or pad byte instead of returning error.

### P1.6 WAV chunk parsing
`audio/wav.go:61-102` — Skip unknown chunks when seeking `fmt ` and `data` sub-chunks. Handle `Subchunk1Size > 16` (WAV_FORMAT_EXTENSIBLE). Account for odd-sized chunk padding bytes.

### P1.7 Input validation
`flac/encoder.go:34-49,72-117` — Validate block sizes (16-65535), sample rates (fit in 20 bits), bit depths (4-32), channel counts (1-8). Return descriptive errors.

### P1.8 Min/max frame size in STREAMINFO
`flac/encoder.go:189-207` — Write min frame size (bytes 4-6) and max frame size (bytes 7-9) as 24-bit big-endian values, or document they are intentionally 0 (meaning "unknown").

### P1.9 Total samples / MD5 offset conflict
`flac/encoder.go:202,205` — Remove the incorrect `PutUint64` at offset 18 — total samples goes in the packed bitfield at bytes 10-17, not at 18. MD5 should be the only thing at bytes 18-33.

### P1.10 Encode cleanup on failure
`flac/encoder.go:72-117` — If `Encode()` fails partway through, close and remove the partial output file so no corrupt file is left on disk.

### P1.11 Stream footer implementation
`flac/encoder.go:212-217` — `writeStreamFooter` always returns `"not implemented"`, causing every `Encode()` call to fail. Implement it (minimally: no explicit end marker needed after last frame, or write a padding byte).

## Phase 1b: Code Quality & Technical Debt

### P1b.1 Dead code — calcMinBlockSize
`flac/encoder.go:307-317` — Function is defined but never called. Name is misleading — it calculates bytes for exactly 16 samples, not a minimum block size. Either remove it or repurpose.

### P1b.2 Dead code — EncodingError
`flac/errors.go:5-16` — `EncodingError` type, `Error()` method, and `NewEncodingError()` factory are never used. The encoder uses `fmt.Errorf` directly. Either integrate or remove.

### P1b.3 Stub functions returning nil
`flac/encoder.go:254-282` — `predictSamples` and `encodeResidual` return nil slices. No callers currently, but would cause nil pointer panics. Add guards or implement.

### P1b.4 Duplicate doc comment
`flac/encoder.go:51-71` — Duplicates the package-level doc. Remove or consolidate into proper Go-style doc comments.

### P1b.5 Typo and capitalization
- `flac/encoder.go:40` — "requriemnts" → "requirements"
- `flac/encoder.go:40,41,128,152,284,298` — "flac" → "FLAC" in comments

### P1b.6 notes.md is gitignored but still tracked
`.gitignore:5` — `notes.md` is in `.gitignore` but was committed early and remains tracked. Either `git rm --cached notes.md` and consolidate into PLAN.md, or remove notes.md content.

## Phase 1c: WAV Parser Issues

### P1c.1 Redundant os.Stat
`audio/wav.go:42-44` — `NewWAVFormat` calls `os.Stat` before `os.Open`. The `os.Open` will fail on its own. Remove the redundant `os.Stat` call (and eliminate a TOCTOU race).

### P1c.2 Partial reads not handled
`audio/wav.go:132-137` — `ReadSamples` calls `file.Read()` but doesn't loop to fill the buffer per the `io.Reader` contract. Use `io.ReadFull` or loop until buffer is filled or EOF.

### P1c.3 Zero-length buffer silent return
`audio/wav.go:125-146` — If `len(buffer) == 0`, `ReadSamples` returns 0 samples with no error. Could cause infinite loops in callers. Handle or document.

### P1c.4 Integer division truncation
`audio/wav.go:137` — If `n % bytesPerSample != 0` (corrupt file), the last partial sample is silently dropped. Validate and return an error.

### P1c.5 No WAV_FORMAT_EXTENSIBLE support
`audio/wav.go:23` — Only handles `AudioFormat == 1` (PCM). Many modern WAV files use `0xFFFE` (WAV_FORMAT_EXTENSIBLE) with a different sub-chunk structure. Handle `0xFFFE` by reading extra format bytes and extracting the actual PCM format.

### P1c.6 bytesToInt32 no bounds checking
`audio/wav.go:148-171` — No check that the input slice has sufficient length for the given bit depth. Index-out-of-range panic on malformed data. Add length checks.

### P1c.7 Performance: new buffer per ReadSamples call
`audio/wav.go:130` — `ReadSamples` allocates a fresh `bytesBuffer` on every call (GC pressure). Accept a reusable buffer parameter or cache inside `WAVFormat`.

### P1c.8 Performance: binary.Uint16/Uint32 per sample
`audio/wav.go:156,166` — `bytesToInt32` calls `binary.LittleEndian.Uint16`/`Uint32` for every sample. Inline with bit shifts:
- 16-bit: `int32(int16(bytes[0]) | int16(bytes[1])<<8)`
- 32-bit: `int32(bytes[0]) | int32(bytes[1])<<8 | int32(bytes[2])<<16 | int32(bytes[3])<<24`

## Phase 1d: Encoder Performance

### P1d.1 binary.Write per sample uses reflection
`flac/encoder.go:230-235` — `encodeBlock` calls `binary.Write(e.output, binary.LittleEndian, sample)` for every int32 sample. This involves heavy reflection. Use a pre-allocated byte buffer with `binary.LittleEndian.PutUint32` in a loop, then write the entire buffer at once.

### P1d.2 No mutex protection
`flac/encoder.go:22-29` — `Encoder` fields (`input`, `output`, `md5sum`, etc.) have no synchronization. Add a `sync.Mutex` if concurrent access is needed.

## Phase 1e: Test Fixes

### P1e.1 Invalid_minBlockSize test mismatch
`flac/encoder_test.go:107-117` — `TestWriteStreamInfo/Invalid_minBlockSize` sets `minBlockSize: 0` and `expectedErr: true`, but `writeStreamInfo` does no validation. Either add validation to `writeStreamInfo` or fix the test.

### P1e.2 "Invalid audio format" test uses nonexistent file
`flac/encoder_test.go:30-36` — The test case named "Invalid audio format" uses `../sample.txt` which doesn't exist. It tests a missing file, not a bad format. Create a malformed WAV or rename the test.

### P1e.3 Hardcoded output filename
`flac/encoder_test.go:22,39,69` — `TestNewEncoder` uses `"test_output.flac"` instead of a temp directory. Use `os.CreateTemp` or `t.TempDir()`.

### P1e.4 Test mutates input struct
`flac/encoder_test.go:60` — Line `tt.expectedInput = audioFormat` in TestNewEncoder mutates the test case struct. Code smell; use a separate variable.

### P1e.5 Unused constant
`audio/wav_test.go:17` — `expectedsubChunkTwo = "data"` is declared but never used.

### P1e.6 Missing WAV tests
No tests for: `ReadSamples`, `TotalSamples`, `Close`, `Channels`, `BitDepth`, `SampleRate`, `bytesToInt32`. Add unit tests for each, covering all bit depths, edge cases, and error paths.

### P1e.7 Missing Encode tests
No tests for the `Encode()` method. Add integration tests that encode `sample.wav` to `.flac` and verify the output.

## Phase 2: Real Compression

Implement actual FLAC compression to reduce file size.

### P2.1 Fixed prediction
Orders 0-4 per FLAC spec:
- Order 0: predict 0
- Order 1: predict previous sample
- Order 2: predict 2*a(n-1) - a(n-2)
- Order 3: predict 3*a(n-1) - 3*a(n-2) + a(n-3)
- Order 4: predict 4*a(n-1) - 6*a(n-2) + 4*a(n-3) - a(n-4)

### P2.2 Rice coding (residual coding)
Implement partitioned Rice coding with parameter selection.

### P2.3 Subframe type selection
Choose between CONSTANT, VERBATIM, FIXED based on which compresses best.

### P2.4 LPC prediction (optional)
Levinson-Durbin algorithm for higher-order prediction.

### P2.5 Interchannel decorrelation
Independent, left-side, right-side, and mid-side stereo modes.

### P2.6 CRC-8 / CRC-16
Header CRC-8 and frame footer CRC-16 for error detection.

## Phase 3: CLI & Polish

### P3.1 Command-line interface
`cmd/flacenc/main.go` using `flag` package.

### P3.2 Progress reporting
Show percentage, compression ratio, and estimated time.

### P3.3 SEEKTABLE metadata
Allow seeking within encoded files.

### P3.4 Unit tests
Comprehensive tests for frame encoding, subframes, Rice coding.

## Phase 4: FLAC Decoder

### P4.1 Frame header parsing
Sync code detection, block size decoding, sample rate decoding.

### P4.2 Subframe decoding
CONSTANT, VERBATIM, FIXED, LPC subframe decompression.

### P4.3 Rice residual decoding
Partitioned Rice decoding with parameter extraction.

### P4.4 Interchannel undecorrelation
Reconstruct independent channels from side/mid-side encoding.

### P4.5 MD5 verification
Verify decoded audio matches the STREAMINFO MD5 checksum.

## Phase 5: Streaming Support

### P5.1 Streaming interface
Define a packetized frame format with metadata headers.

### P5.2 Chunked encoding
Produce independently decodable frame chunks.

### P5.3 Network transport
TCP or HTTP-based streaming server/client.

### P5.4 Seeking in streams
Use SEEKTABLE for random access within a stream.

## Phase 6: TUI Player

### P6.1 Framework
Use `bubbletea` (Elm-architecture) or `tview` (widget-based).

### P6.2 Local playback
Decode FLAC files and play via an audio backend (`oto`, `beep`, or portaudio).

### P6.3 Streaming playback
Receive and decode frames from a network stream in real-time.

### P6.4 Controls
Play, pause, seek, volume, and playlist management.

### P6.5 Metadata display
Show track info, bitrate, sample rate, and compression stats.
