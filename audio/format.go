package audio

type Format interface {
	SampleRate() int
	Channels() int
	BitDepth() int
	TotalSamples() uint64
	ReadSamples([]int32) (int, error)
}
