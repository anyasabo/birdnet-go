package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tphakala/birdnet-go/internal/classifier"
	"github.com/tphakala/birdnet-go/internal/conf"
)

func Command(settings *conf.Settings) *cobra.Command {
	return &cobra.Command{
		Use:   "benchmark",
		Short: "Run BirdNET inference benchmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmark(settings)
		},
	}
}

func runBenchmark(settings *conf.Settings) error {
	// Initialize orchestrator to resolve the active model and its spec.
	bn, err := classifier.NewOrchestrator(settings)
	if err != nil {
		return fmt.Errorf("failed to initialize BirdNET: %w", err)
	}
	defer bn.Delete()

	spec := bn.ModelInfo.Spec
	sampleSize := spec.SampleRate * int(spec.ClipLength.Seconds())
	clipDuration := spec.ClipLength

	fmt.Printf("Model: %s (%s)\n", bn.ModelInfo.Name, bn.ModelInfo.Backend)
	fmt.Printf("Audio: %d Hz, %v clips (%d samples)\n\n", spec.SampleRate, clipDuration, sampleSize)

	isONNX := bn.ModelInfo.Backend == classifier.BackendONNX

	if isONNX {
		// ONNX models don't use XNNPACK -- run a single benchmark.
		var results benchmarkResults
		fmt.Println("Running ONNX inference benchmark...")
		if err := runInferenceBenchmark(bn, sampleSize, clipDuration, &results); err != nil {
			return fmt.Errorf("ONNX benchmark failed: %w", err)
		}
		printSingleResult("ONNX", &results, clipDuration)
	} else {
		// TFLite: compare XNNPACK vs standard CPU.
		var xnnpackResults, standardResults benchmarkResults

		fmt.Println("Testing with XNNPACK delegate:")
		settings.BirdNET.UseXNNPACK = true
		// Re-create orchestrator with XNNPACK enabled.
		bn.Delete()
		bn, err = classifier.NewOrchestrator(settings)
		if err != nil {
			fmt.Printf("XNNPACK benchmark failed: %v\n", err)
		} else {
			if err := runInferenceBenchmark(bn, sampleSize, clipDuration, &xnnpackResults); err != nil {
				fmt.Printf("XNNPACK benchmark failed: %v\n", err)
			}
			bn.Delete()
		}

		fmt.Println("\nTesting standard CPU inference:")
		settings.BirdNET.UseXNNPACK = false
		bn, err = classifier.NewOrchestrator(settings)
		if err != nil {
			return fmt.Errorf("standard CPU inference benchmark failed: %w", err)
		}
		if err := runInferenceBenchmark(bn, sampleSize, clipDuration, &standardResults); err != nil {
			return fmt.Errorf("standard CPU inference benchmark failed: %w", err)
		}

		printComparison(&xnnpackResults, &standardResults, clipDuration)
	}

	return nil
}

type benchmarkResults struct {
	totalInferences     int
	avgTime             time.Duration
	inferencesPerSecond float64
}

func runInferenceBenchmark(bn *classifier.Orchestrator, sampleSize int, clipDuration time.Duration, results *benchmarkResults) error {
	silentChunk := make([]float32, sampleSize)

	duration := 30 * time.Second
	startTime := time.Now()
	var totalInferences int
	var totalDuration time.Duration

	fmt.Println("Running benchmark for 30 seconds...")

	for time.Since(startTime) < duration {
		inferenceStart := time.Now()
		_, err := bn.Predict(context.Background(), [][]float32{silentChunk})
		if err != nil {
			return fmt.Errorf("prediction failed: %w", err)
		}
		inferenceTime := time.Since(inferenceStart)
		totalDuration += inferenceTime
		totalInferences++

		if totalInferences%10 == 0 {
			avgTime := totalDuration / time.Duration(totalInferences)
			fmt.Printf("\r  Inferences: %d, Average time: %dms",
				totalInferences, avgTime.Milliseconds())
		}
	}
	fmt.Println()

	results.totalInferences = totalInferences
	results.avgTime = totalDuration / time.Duration(totalInferences)
	results.inferencesPerSecond = float64(totalInferences) / duration.Seconds()

	return nil
}

func printSingleResult(backend string, results *benchmarkResults, clipDuration time.Duration) {
	fmt.Printf("\nResults:\n")
	fmt.Printf("Method         Inference Time   Throughput\n")
	fmt.Printf("─────────────  ───────────────  ──────────────────────\n")
	fmt.Printf("%-14s %6.1f ms         %6.2f inferences/sec\n",
		backend,
		float64(results.avgTime.Milliseconds()),
		results.inferencesPerSecond)
	fmt.Printf("─────────────  ───────────────  ──────────────────────\n")

	rating, description := getPerformanceRating(float64(results.avgTime.Milliseconds()), clipDuration)
	fmt.Printf("\nSystem Rating: %s, %s\n", rating, description)
}

func printComparison(xnnpack, standard *benchmarkResults, clipDuration time.Duration) {
	fmt.Printf("\nResults:\n")
	fmt.Printf("Method         Inference Time   Throughput\n")
	fmt.Printf("─────────────  ───────────────  ──────────────────────\n")

	if standard.totalInferences > 0 {
		fmt.Printf("Standard       %6.1f ms         %6.2f inferences/sec\n",
			float64(standard.avgTime.Milliseconds()),
			standard.inferencesPerSecond)
	} else {
		fmt.Printf("Standard       Failed\n")
	}

	if xnnpack.totalInferences > 0 {
		fmt.Printf("XNNPACK        %6.1f ms         %6.2f inferences/sec\n",
			float64(xnnpack.avgTime.Milliseconds()),
			xnnpack.inferencesPerSecond)
	} else {
		fmt.Printf("XNNPACK        Failed\n")
	}
	fmt.Printf("─────────────  ───────────────  ──────────────────────\n")

	if xnnpack.totalInferences > 0 && standard.totalInferences > 0 {
		speedImprovement := (float64(standard.avgTime.Milliseconds()) -
			float64(xnnpack.avgTime.Milliseconds())) /
			float64(standard.avgTime.Milliseconds()) * 100

		fmt.Printf("\nSpeed improvement with XNNPACK: %.1f%%\n", speedImprovement)

		rating, description := getPerformanceRating(float64(xnnpack.avgTime.Milliseconds()), clipDuration)
		fmt.Printf("System Rating: %s, %s\n", rating, description)
	}
}

// getPerformanceRating assesses whether the system can keep up with real-time
// analysis. The budget is the model's clip duration (e.g., 3s for v2.4, 5s for v3.0).
func getPerformanceRating(inferenceTimeMs float64, clipDuration time.Duration) (rating, description string) {
	budgetMs := float64(clipDuration.Milliseconds())
	ratio := inferenceTimeMs / budgetMs

	switch {
	case ratio > 1.0:
		return "Failed", "System is too slow for real-time detection"
	case ratio > 0.67:
		return "Very Poor", "System is too slow for reliable operation"
	case ratio > 0.33:
		return "Poor", "System may struggle with real-time detection"
	case ratio > 0.17:
		return "Decent", "System should handle real-time detection"
	case ratio > 0.07:
		return "Good", "System will perform well"
	case ratio > 0.03:
		return "Very Good", "System will perform very well"
	case ratio > 0.007:
		return "Excellent", "System will perform excellently"
	default:
		return "Superb", "System will perform exceptionally well"
	}
}
