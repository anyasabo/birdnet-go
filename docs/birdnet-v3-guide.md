# BirdNET v3.0 Model Guide

BirdNET v3.0 is a Developer Preview of the next-generation bird classification model from Cornell Lab of Ornithology and Chemnitz University of Technology. It can run as the **primary model** (replacing v2.4) or **alongside v2.4** for side-by-side comparison.

## Key Differences from v2.4

| Aspect | v2.4 (TFLite) | v3.0 (ONNX) |
|--------|---------------|--------------|
| Sample rate | 48 kHz | 32 kHz |
| Clip length | 3 s (fixed) | 5 s |
| Input samples | 144,000 | 160,000 |
| Species | 6,523 | ~11,000 |
| Outputs | logits only | embeddings (1280-dim) + logits |
| Model format | TFLite FP32 | ONNX FP32, FP16 |
| Range filter | Built-in (embedded) | Uses v3 geomodel (separate download) |

## Prerequisites

BirdNET v3.0 requires:
- ONNX Runtime shared library (ships with Docker images, manual install for native builds)
- The BirdNET v3.0 ONNX model file
- A v3-compatible label file

Docker images include everything needed. Native binary users must install ONNX Runtime separately.

## Installation

### Docker (recommended)

Docker images ship with ONNX Runtime pre-installed. Install the v3 model from the Model Gallery in the Settings UI, or set the config directly:

```yaml
birdnet:
  version: "3.0"
```

The model will auto-download from the gallery on first startup if the path isn't configured explicitly.

### Native Binary

1. Build with ONNX support:
   ```bash
   task linux_amd64 -- -tags onnx
   # or for ARM64:
   task linux_arm64 -- -tags onnx
   ```

2. Install ONNX Runtime:
   ```bash
   # Linux x86_64
   wget https://github.com/microsoft/onnxruntime/releases/download/v1.24.4/onnxruntime-linux-x64-1.24.4.tgz
   tar xzf onnxruntime-linux-x64-1.24.4.tgz
   sudo cp onnxruntime-linux-x64-1.24.4/lib/libonnxruntime.so* /usr/lib/

   # Linux ARM64 (Raspberry Pi)
   wget https://github.com/microsoft/onnxruntime/releases/download/v1.24.4/onnxruntime-linux-aarch64-1.24.4.tgz
   tar xzf onnxruntime-linux-aarch64-1.24.4.tgz
   sudo cp onnxruntime-linux-aarch64-1.24.4/lib/libonnxruntime.so* /usr/lib/

   # macOS (Apple Silicon)
   wget https://github.com/microsoft/onnxruntime/releases/download/v1.24.4/onnxruntime-osx-arm64-1.24.4.tgz
   tar xzf onnxruntime-osx-arm64-1.24.4.tgz
   sudo cp onnxruntime-osx-arm64-1.24.4/lib/libonnxruntime.dylib /usr/local/lib/
   ```

3. Download the model via the gallery UI, or manually:
   - Model: from HuggingFace `tphakala/BirdNET-v3.0`
   - Labels: included alongside the model

## Configuration

### Mode A: v3.0 as Primary Model

Replace v2.4 entirely:

```yaml
birdnet:
  version: "3.0"
  # modelpath and labelpath auto-resolve from gallery install
  # Set explicitly only if you downloaded manually:
  # modelpath: /path/to/birdnet_v3.0.onnx
  # labelpath: /path/to/birdnet_v3.0_labels.txt
```

### Mode B: Side-by-Side Comparison

Run both v2.4 and v3.0 simultaneously:

```yaml
birdnet:
  version: "2.4"  # primary model stays v2.4

models:
  enabled:
    - birdnet       # primary v2.4 (TFLite)
    - birdnet_v3.0  # additional v3.0 (ONNX) for comparison

# Optional: explicit v3 paths (auto-resolved from gallery if omitted)
birdnet_v3:
  # modelpath: /path/to/birdnet_v3.0.onnx
  # labelpath: /path/to/birdnet_v3.0_labels.txt
  # locale: en-uk        # override locale (defaults to primary locale)
  # threshold: 0.8       # confidence threshold for v3 detections
```

In side-by-side mode:
- Both models analyze every audio clip
- v2.4 processes 3-second clips at 48 kHz
- v3.0 processes 5-second clips at 32 kHz
- Detections from each model are stored separately with version tags
- The detection list can be filtered by model version using `?model_version=2.4` or `?model_version=3.0`

### Filtering Detections by Model

The API supports filtering detections by model version:

```
GET /api/v2/detections?model_version=3.0
GET /api/v2/detections?model_version=2.4
```

## Performance Considerations

### Side-by-Side Mode

Inference is serialized (one model runs at a time). Estimated timing per analysis cycle:

| Hardware | v2.4 only | v3.0 only | Side-by-side |
|----------|-----------|-----------|--------------|
| Pi 4 | ~2.9s / 3s clip | ~4-5s / 5s clip | ~7-8s (may skip clips) |
| Pi 5 | ~1.5s / 3s clip | ~2-3s / 5s clip | ~4-5s (real-time capable) |
| x86_64 | <0.5s | <1s | <1.5s |

**Recommendation:** Pi 5 or better for side-by-side mode. Pi 4 users should run v3-only or v2.4-only.

### FP16 on Pi 5

The Pi 5's ARM Cortex-A76 supports FP16 NEON. Using the FP16 model variant can provide ~2x speedup over FP32.

## Range Filter Behavior

- **v2.4 primary:** Uses the embedded TFLite range filter (6,523 species)
- **v3.0 primary or secondary:** Uses the v3 geomodel if installed via the gallery. If unavailable, geographic filtering is disabled and a warning is logged.
- **Side-by-side:** Each model uses its own range filter independently

## Database and Detection Storage

Detections are stored with model metadata:
- v2.4 detections: `model_name = "BirdNET"`, `model_version = "2.4"`
- v3.0 detections: `model_name = "BirdNET"`, `model_version = "3.0"`

Both coexist in the same database. No migration is needed when switching modes.

## Known Limitations (Developer Preview)

- v3.0 is a **Developer Preview** — accuracy and species coverage may change
- Labels are marked "needs cleanup" by upstream
- The 5-second analysis window means longer wait for first detection vs 3s
- No embedded labels for v3 (must be downloaded)
- Variable-length input (shorter clips) is not yet exposed in the config

## Development Setup

For developers working on v3 integration:

```bash
# Build with ONNX tag
task dev_server -- -tags onnx

# Run tests with ONNX tag
go test -tags onnx -race ./internal/inference/onnx/...
go test -tags onnx -race ./internal/classifier/...

# Lint with ONNX tag
golangci-lint run -v --build-tags onnx

# Benchmark comparing backends
task benchmark-onnx
```

The ONNX code is behind `//go:build onnx` build tags. Files:
- `internal/inference/onnx/` — low-level ONNX Runtime wrappers
- `internal/inference/onnx.go` — adapter implementing `inference.Classifier`
- `internal/classifier/model_onnx.go` — BirdNET ONNX initialization
- `internal/classifier/orchestrator_birdnetv3_onnx.go` — v3 side-by-side loader
