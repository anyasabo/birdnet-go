//go:build onnx

package onnx

import (
	"math"
	"testing"
)

func TestSigmoid(t *testing.T) {
	tests := []struct {
		input    float32
		expected float32
	}{
		{0, 0.5},
		{100, 1.0},
		{-100, 0.0},
	}
	for _, tt := range tests {
		got := sigmoid(tt.input)
		if math.Abs(float64(got-tt.expected)) > 1e-5 {
			t.Errorf("sigmoid(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestSigmoidSlice(t *testing.T) {
	input := []float32{0, 10, -10}
	result := sigmoidSlice(input)

	if len(result) != 3 {
		t.Fatalf("got %d results, want 3", len(result))
	}
	if math.Abs(float64(result[0]-0.5)) > 1e-5 {
		t.Errorf("sigmoidSlice[0] = %v, want ~0.5", result[0])
	}
	if result[1] < 0.99 {
		t.Errorf("sigmoidSlice[1] = %v, want ~1.0", result[1])
	}
	if result[2] > 0.01 {
		t.Errorf("sigmoidSlice[2] = %v, want ~0.0", result[2])
	}
}

func TestSoftmax(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		result := softmax(nil)
		if len(result) != 0 {
			t.Errorf("softmax(nil) returned %d elements", len(result))
		}
	})

	t.Run("uniform", func(t *testing.T) {
		input := []float32{1, 1, 1, 1}
		result := softmax(input)
		for i, v := range result {
			if math.Abs(float64(v-0.25)) > 1e-5 {
				t.Errorf("softmax[%d] = %v, want 0.25", i, v)
			}
		}
	})

	t.Run("sums_to_one", func(t *testing.T) {
		input := []float32{1, 2, 3, 4, 5}
		result := softmax(input)
		var sum float32
		for _, v := range result {
			sum += v
		}
		if math.Abs(float64(sum-1.0)) > 1e-5 {
			t.Errorf("softmax sum = %v, want 1.0", sum)
		}
	})

	t.Run("monotonic", func(t *testing.T) {
		input := []float32{1, 2, 3}
		result := softmax(input)
		if result[0] >= result[1] || result[1] >= result[2] {
			t.Errorf("softmax not monotonically increasing: %v", result)
		}
	})
}

func TestTopK(t *testing.T) {
	scores := []float32{0.1, 0.9, 0.5, 0.3, 0.7}
	labels := []string{"A", "B", "C", "D", "E"}

	t.Run("top_3", func(t *testing.T) {
		preds := topK(scores, labels, 3, 0.0)
		if len(preds) != 3 {
			t.Fatalf("got %d predictions, want 3", len(preds))
		}
		if preds[0].Species != "B" || preds[1].Species != "E" || preds[2].Species != "C" {
			t.Errorf("wrong ordering: %v", preds)
		}
	})

	t.Run("min_confidence", func(t *testing.T) {
		preds := topK(scores, labels, 10, 0.5)
		if len(preds) != 3 {
			t.Fatalf("got %d predictions with minConf=0.5, want 3", len(preds))
		}
		for _, p := range preds {
			if p.Confidence < 0.5 {
				t.Errorf("prediction %s has confidence %v below threshold", p.Species, p.Confidence)
			}
		}
	})

	t.Run("k_zero_returns_all", func(t *testing.T) {
		preds := topK(scores, labels, 0, 0.0)
		if len(preds) != 5 {
			t.Fatalf("got %d predictions with k=0, want 5", len(preds))
		}
	})

	t.Run("mismatched_lengths", func(t *testing.T) {
		short := []string{"A", "B"}
		preds := topK(scores, short, 10, 0.0)
		if len(preds) != 2 {
			t.Fatalf("got %d predictions, want 2 (min of scores and labels)", len(preds))
		}
	})
}
