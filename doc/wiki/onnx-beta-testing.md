# ONNX / BirdNET v3.0 Beta Testing

BirdNET-Go includes experimental support for ONNX models, including BirdNET v3.0. This page explains how to try it.

## Prerequisites

- BirdNET-Go Docker image (ONNX support is compiled in) **or** a local build with `-tags onnx`
- ONNX Runtime shared library (`libonnxruntime.so` on Linux, `onnxruntime.dll` on Windows)
- A compatible ONNX model and label file

## Getting the model

### BirdNET v3.0 (preview)

Download from Zenodo when available, or convert from the official BirdNET repository using [birdnet-onnx-converter](https://github.com/tphakala/birdnet-onnx-converter).

### BirdNET v2.4 (ONNX)

Convert the existing TFLite model to ONNX format using `birdnet-onnx-converter`:

```bash
# Clone the converter
git clone https://github.com/tphakala/birdnet-onnx-converter
cd birdnet-onnx-converter

# Convert (requires Python + tf2onnx)
python convert.py --model path/to/BirdNET_GLOBAL_6K_V2.4_Model_FP32.tflite
```

## Getting ONNX Runtime

Download the appropriate release from [Microsoft's GitHub releases](https://github.com/microsoft/onnxruntime/releases):

```bash
# Linux x86_64
curl -fsSL https://github.com/microsoft/onnxruntime/releases/download/v1.24.4/onnxruntime-linux-x64-1.24.4.tgz \
  -o onnxruntime.tgz
tar -xzf onnxruntime.tgz
export ONNX_RUNTIME_PATH=$(pwd)/onnxruntime-linux-x64-1.24.4/lib/libonnxruntime.so

# Linux ARM64
curl -fsSL https://github.com/microsoft/onnxruntime/releases/download/v1.24.4/onnxruntime-linux-aarch64-1.24.4.tgz \
  -o onnxruntime.tgz
tar -xzf onnxruntime.tgz
export ONNX_RUNTIME_PATH=$(pwd)/onnxruntime-linux-aarch64-1.24.4/lib/libonnxruntime.so
```

## Configuration

Set these in your `config.yaml`:

```yaml
birdnet:
  version: "3.0"           # Use BirdNET v3.0 registry entry
  modelpath: /path/to/birdnet_v3.0.onnx
  labelpath: /path/to/labels.txt
  onnxruntimepath: /path/to/libonnxruntime.so
```

Or for the Docker image, mount the model and library:

```bash
docker run -v /path/to/models:/models \
  -v /path/to/libonnxruntime.so:/usr/lib/libonnxruntime.so \
  -e BIRDNET_MODELPATH=/models/birdnet_v3.0.onnx \
  -e BIRDNET_LABELPATH=/models/labels.txt \
  ghcr.io/tphakala/birdnet-go:latest
```

## Running the benchmark

Validate your setup by running the built-in benchmark:

```bash
# Using the Taskfile convenience task
task benchmark-onnx MODEL_PATH=/path/to/model.onnx LABEL_PATH=/path/to/labels.txt

# Or directly
birdnet-go benchmark
```

The benchmark will auto-detect the model type (v2.4 vs v3.0 vs Perch) from the ONNX tensor shapes and report inference timing against the model's real-time budget.

## Known limitations

- **Experimental**: ONNX support has not been tested as extensively as TFLite
- **No GPU acceleration**: Only CPU execution provider is currently supported
- **v3.0 species count**: Determined at runtime from the label file (not hardcoded)
- **Range filter**: The BirdNET meta model (location-based filtering) requires a separate ONNX model file

## Reporting issues

When filing issues related to ONNX, please include:

1. Model file name and source (e.g., "v3.0 from Zenodo" or "converted with birdnet-onnx-converter")
2. ONNX Runtime version
3. Output of `birdnet-go benchmark`
4. Platform and architecture (`uname -a`)
