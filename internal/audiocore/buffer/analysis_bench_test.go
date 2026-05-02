package buffer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tphakala/birdnet-go/internal/audiocore/buffer"
)

const (
	// Realistic BirdNET-Go analysis buffer dimensions.
	benchCapacity    = 288768 * 4 // ~4 windows of headroom
	benchOverlapSize = 144384     // 1.5s overlap at 48kHz 16-bit mono
	benchReadSize    = 144384     // 1.5s fresh data per read
	benchChunkSize   = 2880       // 30ms capture frame at 48kHz 16-bit mono
)

// BenchmarkAnalysisBuffer_WriteRead measures a steady-state write+read cycle:
// write enough capture frames to fill one analysis window, then read. This is
// the pattern the analysis monitor executes on every detection interval.
func BenchmarkAnalysisBuffer_WriteRead(b *testing.B) {
	log := newTestLogger()

	b.Run("unpooled", func(b *testing.B) {
		ab, err := buffer.NewAnalysisBuffer(benchCapacity, benchOverlapSize, benchReadSize, "bench", log, nil)
		require.NoError(b, err)

		chunk := make([]byte, benchChunkSize)
		for i := range chunk {
			chunk[i] = byte(i)
		}

		// Pre-fill so reads can succeed immediately.
		writesPerWindow := benchReadSize / benchChunkSize
		for range writesPerWindow + 1 {
			require.NoError(b, ab.Write(chunk))
		}

		b.SetBytes(int64(benchOverlapSize + benchReadSize))
		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			// Write one window worth of data.
			for range writesPerWindow {
				_ = ab.Write(chunk)
			}
			data, release, err := ab.Read()
			if err != nil {
				b.Fatal(err)
			}
			_ = data
			release()
		}
	})

	b.Run("pooled", func(b *testing.B) {
		mgr := buffer.NewManager(log)
		ab, err := buffer.NewAnalysisBuffer(benchCapacity, benchOverlapSize, benchReadSize, "bench", log, mgr)
		require.NoError(b, err)

		chunk := make([]byte, benchChunkSize)
		for i := range chunk {
			chunk[i] = byte(i)
		}

		writesPerWindow := benchReadSize / benchChunkSize
		for range writesPerWindow + 1 {
			require.NoError(b, ab.Write(chunk))
		}

		b.SetBytes(int64(benchOverlapSize + benchReadSize))
		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			for range writesPerWindow {
				_ = ab.Write(chunk)
			}
			data, release, err := ab.Read()
			if err != nil {
				b.Fatal(err)
			}
			_ = data
			release()
		}
	})
}

// BenchmarkAnalysisBuffer_WriteOnly measures raw write throughput into the ring
// buffer, isolating the mutex + ring.Write cost from the read path.
func BenchmarkAnalysisBuffer_WriteOnly(b *testing.B) {
	log := newTestLogger()
	ab, err := buffer.NewAnalysisBuffer(benchCapacity, benchOverlapSize, benchReadSize, "bench", log, nil)
	require.NoError(b, err)

	chunk := make([]byte, benchChunkSize)
	for i := range chunk {
		chunk[i] = byte(i)
	}

	b.SetBytes(int64(benchChunkSize))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = ab.Write(chunk)
	}
}
