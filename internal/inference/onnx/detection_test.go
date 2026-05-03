//go:build onnx

package onnx

import (
	"testing"
)

func TestDetectModelTypeFromShapes(t *testing.T) {
	tests := []struct {
		name       string
		shapes     [][]int64
		numOutputs int
		wantType   ModelType
		wantErr    bool
	}{
		{
			name:       "BirdNET_v2.4",
			shapes:     [][]int64{{1, 144000}},
			numOutputs: 1,
			wantType:   BirdNETv24,
		},
		{
			name:       "BirdNET_v3.0",
			shapes:     [][]int64{{1, 160000}},
			numOutputs: 2,
			wantType:   BirdNETv30,
		},
		{
			name:       "Perch_v2",
			shapes:     [][]int64{{1, 160000}},
			numOutputs: 4,
			wantType:   PerchV2,
		},
		{
			name:       "unknown_sample_count",
			shapes:     [][]int64{{1, 100000}},
			numOutputs: 1,
			wantErr:    true,
		},
		{
			name:       "unknown_output_count",
			shapes:     [][]int64{{1, 144000}},
			numOutputs: 3,
			wantErr:    true,
		},
		{
			name:       "empty_shapes",
			shapes:     nil,
			numOutputs: 1,
			wantErr:    true,
		},
		{
			name:       "too_few_dimensions",
			shapes:     [][]int64{{144000}},
			numOutputs: 1,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := detectModelTypeFromShapes(tt.shapes, tt.numOutputs)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mt != tt.wantType {
				t.Errorf("got %v, want %v", mt, tt.wantType)
			}
		})
	}
}

func TestBuildModelConfig(t *testing.T) {
	t.Run("v24", func(t *testing.T) {
		cfg := buildModelConfig(BirdNETv24, []int64{1, 144000}, 1)
		if cfg.SampleRate != 48000 {
			t.Errorf("SampleRate = %d, want 48000", cfg.SampleRate)
		}
		if cfg.SampleCount != 144000 {
			t.Errorf("SampleCount = %d, want 144000", cfg.SampleCount)
		}
		if cfg.LogitsIndex != 0 {
			t.Errorf("LogitsIndex = %d, want 0", cfg.LogitsIndex)
		}
		if cfg.EmbeddingIndex != -1 {
			t.Errorf("EmbeddingIndex = %d, want -1", cfg.EmbeddingIndex)
		}
	})

	t.Run("v30", func(t *testing.T) {
		cfg := buildModelConfig(BirdNETv30, []int64{1, 160000}, 2)
		if cfg.SampleRate != 32000 {
			t.Errorf("SampleRate = %d, want 32000", cfg.SampleRate)
		}
		if cfg.Duration != 5.0 {
			t.Errorf("Duration = %v, want 5.0", cfg.Duration)
		}
		if cfg.LogitsIndex != 1 {
			t.Errorf("LogitsIndex = %d, want 1", cfg.LogitsIndex)
		}
		if cfg.EmbeddingIndex != 0 {
			t.Errorf("EmbeddingIndex = %d, want 0", cfg.EmbeddingIndex)
		}
		if cfg.EmbeddingSize != 1280 {
			t.Errorf("EmbeddingSize = %d, want 1280", cfg.EmbeddingSize)
		}
	})

	t.Run("perch", func(t *testing.T) {
		cfg := buildModelConfig(PerchV2, []int64{1, 160000}, 4)
		if cfg.LogitsIndex != 3 {
			t.Errorf("LogitsIndex = %d, want 3", cfg.LogitsIndex)
		}
		if cfg.EmbeddingSize != 1536 {
			t.Errorf("EmbeddingSize = %d, want 1536", cfg.EmbeddingSize)
		}
	})
}

func TestModelType_Methods(t *testing.T) {
	tests := []struct {
		mt          ModelType
		sampleRate  int
		duration    float64
		sampleCount int
		str         string
	}{
		{BirdNETv24, 48000, 3.0, 144000, "BirdNET v2.4"},
		{BirdNETv30, 32000, 5.0, 160000, "BirdNET v3.0"},
		{PerchV2, 32000, 5.0, 160000, "Perch v2"},
		{ModelType(99), 0, 0, 0, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			if got := tt.mt.SampleRate(); got != tt.sampleRate {
				t.Errorf("SampleRate() = %d, want %d", got, tt.sampleRate)
			}
			if got := tt.mt.Duration(); got != tt.duration {
				t.Errorf("Duration() = %v, want %v", got, tt.duration)
			}
			if got := tt.mt.SampleCount(); got != tt.sampleCount {
				t.Errorf("SampleCount() = %d, want %d", got, tt.sampleCount)
			}
			if got := tt.mt.String(); got != tt.str {
				t.Errorf("String() = %q, want %q", got, tt.str)
			}
		})
	}
}
