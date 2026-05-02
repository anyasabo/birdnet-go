package convert_test

import (
	"testing"

	"github.com/tphakala/birdnet-go/internal/audiocore/convert"
)

// makeBenchPCM16 builds a synthetic 16-bit PCM byte slice with sampleCount
// samples. Values ramp through the full int16 range to exercise realistic
// conversion arithmetic (not silent audio).
func makeBenchPCM16(sampleCount int) []byte {
	buf := make([]byte, sampleCount*2)
	for i := range sampleCount {
		v := int16((i % 65536) - 32768)
		buf[i*2] = byte(v)
		buf[i*2+1] = byte(v >> 8)
	}
	return buf
}

// makeFloat64Samples builds a normalised float64 slice for PCM16 round-trip
// benchmarks. Values span [-1.0, 1.0].
func makeFloat64Samples(count int) []float64 {
	out := make([]float64, count)
	for i := range count {
		out[i] = float64(i)/float64(count)*2.0 - 1.0
	}
	return out
}

// BenchmarkConvert16BitToFloat32 measures the allocating 16-bit PCM→float32
// path used by non-pooled callers (audiocore/convert.ConvertToFloat32).
func BenchmarkConvert16BitToFloat32(b *testing.B) {
	sizes := []struct {
		name    string
		samples int
	}{
		{"20ms_48kHz", 960},
		{"100ms_48kHz", 4800},
		{"3s_48kHz", 144384},
	}

	for _, sz := range sizes {
		pcm := makeBenchPCM16(sz.samples)
		b.Run(sz.name, func(b *testing.B) {
			b.SetBytes(int64(len(pcm)))
			b.ReportAllocs()
			for b.Loop() {
				result, _ := convert.ConvertToFloat32(pcm, 16)
				_ = result
			}
		})
	}
}

// BenchmarkConvert24BitToFloat32 measures 24-bit PCM→float32 conversion.
func BenchmarkConvert24BitToFloat32(b *testing.B) {
	sampleCount := 144384
	pcm := make([]byte, sampleCount*3)
	for i := range sampleCount {
		v := int32((i % 65536) - 32768)
		pcm[i*3] = byte(v)
		pcm[i*3+1] = byte(v >> 8)
		pcm[i*3+2] = byte(v >> 16)
	}

	b.SetBytes(int64(len(pcm)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result, _ := convert.ConvertToFloat32(pcm, 24)
		_ = result
	}
}

// BenchmarkBytesToFloat64PCM16Into measures the zero-alloc byte→float64
// conversion used by the EQ/gain path in the audio router.
func BenchmarkBytesToFloat64PCM16Into(b *testing.B) {
	sizes := []struct {
		name    string
		samples int
	}{
		{"20ms_48kHz", 960},
		{"100ms_48kHz", 4800},
		{"3s_48kHz", 144384},
	}

	for _, sz := range sizes {
		pcm := makeBenchPCM16(sz.samples)
		dst := make([]float64, sz.samples)
		b.Run(sz.name, func(b *testing.B) {
			b.SetBytes(int64(len(pcm)))
			b.ReportAllocs()
			for b.Loop() {
				convert.BytesToFloat64PCM16Into(dst, pcm)
			}
		})
	}
}

// BenchmarkFloat64ToBytesPCM16 measures float64→byte conversion with SIMD
// clamping. Note: this function modifies the input slice in-place during
// clamping, so we reset it each iteration.
func BenchmarkFloat64ToBytesPCM16(b *testing.B) {
	sizes := []struct {
		name    string
		samples int
	}{
		{"20ms_48kHz", 960},
		{"100ms_48kHz", 4800},
		{"3s_48kHz", 144384},
	}

	for _, sz := range sizes {
		src := makeFloat64Samples(sz.samples)
		pristine := make([]float64, len(src))
		copy(pristine, src)
		out := make([]byte, sz.samples*2)

		b.Run(sz.name, func(b *testing.B) {
			b.SetBytes(int64(len(out)))
			b.ReportAllocs()
			for b.Loop() {
				copy(src, pristine)
				_ = convert.Float64ToBytesPCM16(src, out)
			}
		})
	}
}

// BenchmarkBytesToFloat64PCM16_Allocating measures the allocating variant
// for comparison with the Into (pre-allocated) version.
func BenchmarkBytesToFloat64PCM16_Allocating(b *testing.B) {
	pcm := makeBenchPCM16(144384)
	b.SetBytes(int64(len(pcm)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result := convert.BytesToFloat64PCM16(pcm)
		_ = result
	}
}
