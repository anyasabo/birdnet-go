//go:build onnx

package onnx

import (
	"math"
	"testing"
)

func TestCalculateWeek(t *testing.T) {
	tests := []struct {
		month, day int
		want       float32
	}{
		{1, 1, 1},
		{1, 7, 1},
		{1, 8, 2},
		{1, 14, 2},
		{1, 15, 3},
		{1, 21, 3},
		{1, 22, 4},
		{1, 28, 4},
		{1, 29, 4}, // days 29-31 clamp to week 4
		{1, 31, 4},
		{2, 1, 5},
		{12, 31, 48},
	}
	for _, tt := range tests {
		got := CalculateWeek(tt.month, tt.day)
		if math.Abs(float64(got-tt.want)) > 1e-6 {
			t.Errorf("CalculateWeek(%d, %d) = %v, want %v", tt.month, tt.day, got, tt.want)
		}
	}
}

func TestValidateCoordinates(t *testing.T) {
	tests := []struct {
		lat, lon float32
		wantErr  bool
	}{
		{0, 0, false},
		{90, 180, false},
		{-90, -180, false},
		{91, 0, true},
		{-91, 0, true},
		{0, 181, true},
		{0, -181, true},
	}
	for _, tt := range tests {
		err := ValidateCoordinates(tt.lat, tt.lon)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateCoordinates(%v, %v) err=%v, wantErr=%v", tt.lat, tt.lon, err, tt.wantErr)
		}
	}
}

func TestValidateDate(t *testing.T) {
	tests := []struct {
		month, day int
		wantErr    bool
	}{
		{1, 1, false},
		{12, 31, false},
		{6, 15, false},
		{0, 1, true},
		{13, 1, true},
		{1, 0, true},
		{1, 32, true},
	}
	for _, tt := range tests {
		err := ValidateDate(tt.month, tt.day)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateDate(%d, %d) err=%v, wantErr=%v", tt.month, tt.day, err, tt.wantErr)
		}
	}
}

func TestFilterPredictions(t *testing.T) {
	predictions := []Prediction{
		{Species: "A", Confidence: 0.9, Index: 0},
		{Species: "B", Confidence: 0.7, Index: 1},
		{Species: "C", Confidence: 0.5, Index: 2},
	}
	scores := []LocationScore{
		{Species: "A", Score: 0.8},
		{Species: "B", Score: 0.01}, // below threshold
		{Species: "C", Score: 0.5},
	}

	t.Run("no_rerank", func(t *testing.T) {
		result := filterPredictions(predictions, scores, 0.03, false)
		if len(result) != 2 {
			t.Fatalf("got %d, want 2 (B filtered out)", len(result))
		}
		if result[0].Confidence != 0.9 {
			t.Errorf("confidence should be unchanged without rerank, got %v", result[0].Confidence)
		}
	})

	t.Run("rerank", func(t *testing.T) {
		result := filterPredictions(predictions, scores, 0.03, true)
		if len(result) != 2 {
			t.Fatalf("got %d, want 2", len(result))
		}
		// A: 0.9 * 0.8 = 0.72, C: 0.5 * 0.5 = 0.25
		if result[0].Species != "A" || result[1].Species != "C" {
			t.Errorf("unexpected order: %v", result)
		}
		if math.Abs(float64(result[0].Confidence-0.72)) > 1e-5 {
			t.Errorf("reranked A confidence = %v, want 0.72", result[0].Confidence)
		}
	})

	t.Run("unknown_species_filtered", func(t *testing.T) {
		preds := []Prediction{{Species: "X", Confidence: 0.9}}
		result := filterPredictions(preds, scores, 0.03, false)
		if len(result) != 0 {
			t.Errorf("species not in scores should be filtered, got %d", len(result))
		}
	})
}

func TestFilterBatchPredictions(t *testing.T) {
	batches := [][]Prediction{
		{{Species: "A", Confidence: 0.9}},
		{{Species: "B", Confidence: 0.7}},
	}
	scores := []LocationScore{
		{Species: "A", Score: 0.5},
	}
	result := filterBatchPredictions(batches, scores, 0.03, false)
	if len(result) != 2 {
		t.Fatalf("got %d batches, want 2", len(result))
	}
	if len(result[0]) != 1 {
		t.Errorf("batch 0 should have 1 result")
	}
	if len(result[1]) != 0 {
		t.Errorf("batch 1 should be empty (B not in scores)")
	}
}
