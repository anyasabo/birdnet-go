package resample

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// makeBenchPCM16 builds a synthetic 16-bit PCM byte slice. Same ramp pattern
// as makePCM16 in resample_test.go but usable from benchmarks (no *testing.T).
func makeBenchPCM16(sampleCount int) []byte {
	buf := make([]byte, sampleCount*bytesPerSample)
	for i := range sampleCount {
		v := int16((i % 65536) - 32768) //nolint:gosec // G115: intentional narrowing for bench data
		buf[i*2] = byte(v)
		buf[i*2+1] = byte(v >> 8)
	}
	return buf
}

// BenchmarkResampleInto measures steady-state resampling throughput at common
// rate pairs used in BirdNET-Go deployments.
func BenchmarkResampleInto(b *testing.B) {
	cases := []struct {
		fromRate int
		toRate   int
		durLabel string
		samples  int
	}{
		{48000, 32000, "100ms", 4800},
		{48000, 16000, "100ms", 4800},
		{44100, 48000, "100ms", 4410},
		{48000, 32000, "3s", 144000},
		{48000, 16000, "3s", 144000},
	}

	for _, tc := range cases {
		name := fmt.Sprintf("%dHz_to_%dHz/%s", tc.fromRate, tc.toRate, tc.durLabel)
		b.Run(name, func(b *testing.B) {
			r, err := NewResampler(tc.fromRate, tc.toRate)
			require.NoError(b, err)
			require.NotNil(b, r)
			b.Cleanup(func() { _ = r.Close() })

			input := makeBenchPCM16(tc.samples)

			// Warm the resampler so internal buffers are sized.
			_, err = r.ResampleInto(input)
			require.NoError(b, err)

			b.SetBytes(int64(len(input)))
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				out, resErr := r.ResampleInto(input)
				if resErr != nil {
					b.Fatal(resErr)
				}
				_ = out
			}
		})
	}
}

// BenchmarkResampleInto_ColdStart measures the first-call cost when internal
// scratch buffers have not yet been sized.
func BenchmarkResampleInto_ColdStart(b *testing.B) {
	input := makeBenchPCM16(4800) // 100ms at 48kHz

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r, err := NewResampler(48000, 32000)
		if err != nil {
			b.Fatal(err)
		}
		out, err := r.ResampleInto(input)
		if err != nil {
			b.Fatal(err)
		}
		_ = out
		_ = r.Close()
	}
}
