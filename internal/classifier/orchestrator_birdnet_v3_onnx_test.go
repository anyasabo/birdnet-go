package classifier

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tphakala/birdnet-go/internal/conf"
)

func TestModelLoadersIncludesBirdNETv3(t *testing.T) {
	loader, ok := modelLoaders[RegistryIDBirdNETV3]
	assert.True(t, ok, "modelLoaders should contain BirdNET v3.0 entry")
	assert.NotNil(t, loader, "BirdNET v3.0 loader function should not be nil")
}

func TestLoadBirdNETv3_MissingFiles(t *testing.T) {
	settings := &conf.Settings{}
	settings.BirdNET.ONNXRuntimePath = ""

	o := &Orchestrator{
		Settings: settings,
		models:   make(map[string]*modelEntry),
	}

	err := o.loadBirdNETv3(1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not installed or configured")
}

func TestLoadBirdNETv3_ConfigPathsUsed(t *testing.T) {
	settings := &conf.Settings{}
	settings.BirdNETv3.ModelPath = "/nonexistent/model.onnx"
	settings.BirdNETv3.LabelPath = "/nonexistent/labels.txt"
	settings.BirdNET.ONNXRuntimePath = ""

	o := &Orchestrator{
		Settings: settings,
		models:   make(map[string]*modelEntry),
	}

	err := o.loadBirdNETv3(1)
	require.Error(t, err)
	// Should get past the "not installed" check and fail on ORT availability
	assert.Contains(t, err.Error(), "ONNX Runtime")
}
