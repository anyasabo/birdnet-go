// process_bench_test.go benchmarks the components of the ProcessData hot path
// that can be exercised without a TFLite model: PCM-to-float32 pooled
// conversion, the PCM copy for the results queue, and non-blocking queue
// enqueue.
package analysis

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tphakala/birdnet-go/internal/audiocore/buffer"
	"github.com/tphakala/birdnet-go/internal/classifier"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/datastore"
	"github.com/tphakala/birdnet-go/internal/logger"
)

func newBenchLogger() logger.Logger {
	return logger.NewSlogLogger(io.Discard, logger.LogLevelError, time.UTC)
}

// makeBenchPCM builds a synthetic 16-bit PCM byte slice at the standard
// analysis window size (conf.BufferSize bytes = 3s at 48kHz 16-bit mono).
func makeBenchPCM() []byte {
	pcm := make([]byte, conf.BufferSize)
	for i := range pcm {
		pcm[i] = byte(i)
	}
	return pcm
}

// BenchmarkConvertToFloat32WithPool measures the pooled 16-bit PCM-to-float32
// conversion used at the top of ProcessData. This is the same function tested
// for allocation count in process_alloc_test.go, measured here for throughput.
func BenchmarkConvertToFloat32WithPool(b *testing.B) {
	mgr := buffer.NewManager(newBenchLogger())
	pcm := makeBenchPCM()

	// Warm the pool.
	warm := convert16BitToFloat32WithPool(mgr, pcm)
	if p := mgr.Float32PoolFor(len(warm)); p != nil {
		p.Put(warm)
	}

	b.SetBytes(int64(len(pcm)))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		out := convert16BitToFloat32WithPool(mgr, pcm)
		if p := mgr.Float32PoolFor(len(out)); p != nil {
			p.Put(out)
		}
	}
}

// BenchmarkPCMCopy measures the per-window PCM copy that ProcessData performs
// before enqueueing results. This is the largest single allocation in the
// analysis hot path on embedded devices.
func BenchmarkPCMCopy(b *testing.B) {
	pcm := makeBenchPCM()

	b.SetBytes(int64(len(pcm)))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		pcmCopy := make([]byte, len(pcm))
		copy(pcmCopy, pcm)
		_ = pcmCopy
	}
}

// BenchmarkResultsQueueEnqueue measures the non-blocking channel send to the
// classifier results queue, including the Results struct construction. A
// background goroutine drains the queue to prevent it from filling.
func BenchmarkResultsQueueEnqueue(b *testing.B) {
	pcm := makeBenchPCM()
	results := []datastore.Results{
		{Species: "Turdus merula_Common Blackbird", Confidence: 0.95},
	}
	source := datastore.AudioSource{
		ID: "bench", SafeString: "bench", DisplayName: "bench",
	}

	// Drain the queue in background so sends never block.
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-classifier.ResultsQueue:
			}
		}
	}()
	b.Cleanup(func() { close(done) })

	b.SetBytes(int64(len(pcm)))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		pcmCopy := make([]byte, len(pcm))
		copy(pcmCopy, pcm)

		msg := classifier.Results{
			StartTime:       time.Now(),
			AudioCapturedAt: time.Now(),
			ElapsedTime:     100 * time.Millisecond,
			PCMdata:         pcmCopy,
			Results:         results,
			Source:          source,
			ModelID:         "bench-model",
		}

		select {
		case classifier.ResultsQueue <- msg:
		default:
		}
	}
}

// BenchmarkProcessDataComponents measures the combined cost of the Go-side
// work that ProcessData performs around inference: float32 conversion (pooled),
// PCM copy, and queue enqueue. Inference itself is excluded.
func BenchmarkProcessDataComponents(b *testing.B) {
	mgr := buffer.NewManager(newBenchLogger())
	pcm := makeBenchPCM()
	results := []datastore.Results{
		{Species: "Turdus merula_Common Blackbird", Confidence: 0.95},
	}
	source := datastore.AudioSource{
		ID: "bench", SafeString: "bench", DisplayName: "bench",
	}

	// Warm the pool.
	warm := convert16BitToFloat32WithPool(mgr, pcm)
	if p := mgr.Float32PoolFor(len(warm)); p != nil {
		p.Put(warm)
	}

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-classifier.ResultsQueue:
			}
		}
	}()
	b.Cleanup(func() { close(done) })

	b.SetBytes(int64(len(pcm)))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		// 1. Convert PCM to float32 (pooled).
		sampleData := convert16BitToFloat32WithPool(mgr, pcm)

		// 2. Return float32 buffer to pool (simulates post-inference return).
		if p := mgr.Float32PoolFor(len(sampleData)); p != nil {
			p.Put(sampleData)
		}

		// 3. Copy PCM for queue ownership transfer.
		pcmCopy := make([]byte, len(pcm))
		copy(pcmCopy, pcm)

		// 4. Enqueue results.
		msg := classifier.Results{
			StartTime:       time.Now(),
			AudioCapturedAt: time.Now(),
			ElapsedTime:     100 * time.Millisecond,
			PCMdata:         pcmCopy,
			Results:         results,
			Source:          source,
			ModelID:         "bench-model",
		}
		select {
		case classifier.ResultsQueue <- msg:
		default:
		}
	}
}

// BenchmarkConvertToFloat32WithPool_Wrapper measures the exported wrapper
// (convertToFloat32WithPool) which wraps the result in [][]float32{...},
// adding one outer-slice allocation per call.
func BenchmarkConvertToFloat32WithPool_Wrapper(b *testing.B) {
	mgr := buffer.NewManager(newBenchLogger())
	require.NotNil(b, mgr)
	pcm := makeBenchPCM()

	b.SetBytes(int64(len(pcm)))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		result, err := convertToFloat32WithPool(mgr, pcm, 16)
		if err != nil {
			b.Fatal(err)
		}
		if p := mgr.Float32PoolFor(len(result[0])); p != nil {
			p.Put(result[0])
		}
	}
}
