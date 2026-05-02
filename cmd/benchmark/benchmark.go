package benchmark

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/tphakala/birdnet-go/internal/classifier"
	"github.com/tphakala/birdnet-go/internal/conf"
)

var detailed bool

func Command(settings *conf.Settings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Run BirdNET inference benchmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmark(settings)
		},
	}
	cmd.Flags().BoolVar(&detailed, "detailed", false, "report percentile latencies, GC stats, and memory allocation rates")
	return cmd
}

func runBenchmark(settings *conf.Settings) error {
	var xnnpackResults, standardResults benchmarkResults

	// First run with XNNPACK
	fmt.Println("🚀 Testing with XNNPACK delegate:")
	settings.BirdNET.UseXNNPACK = true
	if err := runInferenceBenchmark(settings, &xnnpackResults); err != nil {
		fmt.Printf("❌ XNNPACK benchmark failed: %v\n", err)
	}

	// Then run without XNNPACK
	fmt.Println("\n🐌 Testing standard CPU inference:")
	settings.BirdNET.UseXNNPACK = false
	if err := runInferenceBenchmark(settings, &standardResults); err != nil {
		return fmt.Errorf("❌ standard CPU inference benchmark failed: %w", err)
	}

	// Show detailed performance comparison
	fmt.Printf("Results:\n")
	fmt.Printf("Method         Inference Time   Throughput\n")
	fmt.Printf("─────────────  ───────────────  ──────────────────────\n")

	if standardResults.totalInferences > 0 {
		fmt.Printf("Standard       %6.1f ms         %6.2f inferences/sec\n",
			float64(standardResults.avgTime.Milliseconds()),
			standardResults.inferencesPerSecond)
	} else {
		fmt.Printf("Standard       ❌ Failed\n")
	}

	if xnnpackResults.totalInferences > 0 {
		fmt.Printf("XNNPACK        %6.1f ms         %6.2f inferences/sec\n",
			float64(xnnpackResults.avgTime.Milliseconds()),
			xnnpackResults.inferencesPerSecond)
	} else {
		fmt.Printf("XNNPACK        ❌ Failed\n")
	}
	fmt.Printf("─────────────  ───────────────  ──────────────────────\n")

	if xnnpackResults.totalInferences > 0 && standardResults.totalInferences > 0 {
		speedImprovement := (float64(standardResults.avgTime.Milliseconds()) -
			float64(xnnpackResults.avgTime.Milliseconds())) /
			float64(standardResults.avgTime.Milliseconds()) * 100

		fmt.Printf("\n🚀 Speed improvement with XNNPACK: %.1f%%\n", speedImprovement)

		rating, description := getPerformanceRating(float64(xnnpackResults.avgTime.Milliseconds()))
		fmt.Printf("System Rating: %s, %s\n", rating, description)
	}

	if detailed {
		fmt.Println()
		printDetailedResults("XNNPACK", &xnnpackResults)
		printDetailedResults("Standard", &standardResults)
	}

	return nil
}

type benchmarkResults struct {
	totalInferences     int
	avgTime             time.Duration
	inferencesPerSecond float64
	latencies           []time.Duration
	memBefore           runtime.MemStats
	memAfter            runtime.MemStats
}

func runInferenceBenchmark(settings *conf.Settings, results *benchmarkResults) error {
	bn, err := classifier.NewOrchestrator(settings)
	if err != nil {
		return fmt.Errorf("failed to initialize BirdNET: %w", err)
	}
	defer bn.Delete()

	sampleSize := 48000 * 3
	silentChunk := make([]float32, sampleSize)

	duration := 30 * time.Second

	// Pre-allocate latency slice to avoid measurement noise from append growth.
	// Estimate capacity from duration assuming ~10ms per inference as lower bound.
	estimatedCap := int(duration.Seconds() * 100)
	if detailed {
		results.latencies = make([]time.Duration, 0, estimatedCap)
	}

	// Snapshot memory before the benchmark loop.
	if detailed {
		runtime.GC()
		runtime.ReadMemStats(&results.memBefore)
	}

	startTime := time.Now()
	var totalInferences int
	var totalDuration time.Duration

	fmt.Println("⏳ Running benchmark for 30 seconds...")

	for time.Since(startTime) < duration {
		inferenceStart := time.Now()
		_, predErr := bn.Predict(context.Background(), [][]float32{silentChunk})
		if predErr != nil {
			return fmt.Errorf("prediction failed: %w", predErr)
		}
		inferenceTime := time.Since(inferenceStart)
		totalDuration += inferenceTime
		totalInferences++

		if detailed {
			results.latencies = append(results.latencies, inferenceTime)
		}

		if totalInferences%10 == 0 {
			avgTime := totalDuration / time.Duration(totalInferences)
			fmt.Printf("\r🔄 Inferences: \033[1;36m%d\033[0m, Average time: \033[1;33m%dms\033[0m",
				totalInferences, avgTime.Milliseconds())
		}
	}
	fmt.Println()

	if detailed {
		runtime.ReadMemStats(&results.memAfter)
	}

	results.totalInferences = totalInferences
	results.avgTime = totalDuration / time.Duration(totalInferences)
	results.inferencesPerSecond = float64(totalInferences) / duration.Seconds()

	return nil
}

func printDetailedResults(label string, r *benchmarkResults) {
	if r.totalInferences == 0 {
		return
	}
	fmt.Printf("── %s detailed ──\n", label)

	if len(r.latencies) > 0 {
		sort.Slice(r.latencies, func(i, j int) bool { return r.latencies[i] < r.latencies[j] })
		n := len(r.latencies)
		fmt.Printf("  Latency p50:  %6.1f ms\n", float64(r.latencies[n/2].Microseconds())/1000.0)
		fmt.Printf("  Latency p95:  %6.1f ms\n", float64(percentile(r.latencies, 0.95).Microseconds())/1000.0)
		fmt.Printf("  Latency p99:  %6.1f ms\n", float64(percentile(r.latencies, 0.99).Microseconds())/1000.0)
		fmt.Printf("  Latency min:  %6.1f ms\n", float64(r.latencies[0].Microseconds())/1000.0)
		fmt.Printf("  Latency max:  %6.1f ms\n", float64(r.latencies[n-1].Microseconds())/1000.0)
	}

	allocBytes := r.memAfter.TotalAlloc - r.memBefore.TotalAlloc
	gcRuns := r.memAfter.NumGC - r.memBefore.NumGC
	gcPause := r.memAfter.PauseTotalNs - r.memBefore.PauseTotalNs

	fmt.Printf("  Alloc total:  %6.1f MB  (%d KB/inference)\n",
		float64(allocBytes)/(1024*1024),
		allocBytes/uint64(r.totalInferences)/1024)
	fmt.Printf("  GC runs:      %d  (%.1f ms total pause)\n",
		gcRuns, float64(gcPause)/1e6)
	fmt.Printf("  Heap in-use:  %6.1f MB\n",
		float64(r.memAfter.HeapInuse)/(1024*1024))
	fmt.Println()
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func getPerformanceRating(inferenceTime float64) (rating, description string) {
	switch {
	case inferenceTime > 3000:
		return "❌ Failed", "System is too slow for BirdNET-Go real-time detection"
	case inferenceTime > 2000:
		return "❌ Very Poor", "System is too slow for reliable operation"
	case inferenceTime > 1000:
		return "⚠️ Poor", "System may struggle with real-time detection"
	case inferenceTime > 500:
		return "👍 Decent", "System should handle real-time detection"
	case inferenceTime > 200:
		return "✨ Good", "System will perform well"
	case inferenceTime > 100:
		return "🌟 Very Good", "System will perform very well"
	case inferenceTime > 20:
		return "🏆 Excellent", "System will perform excellently"
	default:
		return "🚀 Superb", "System will perform exceptionally well"
	}
}
