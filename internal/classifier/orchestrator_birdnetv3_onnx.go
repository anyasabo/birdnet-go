package classifier

import (
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/errors"
	"github.com/tphakala/birdnet-go/internal/inference"
	"github.com/tphakala/birdnet-go/internal/logger"
)

// loadBirdNETv3 creates and registers a BirdNET v3.0 ONNX model instance as an
// additional (non-primary) model. This enables side-by-side comparison with the
// primary model (typically v2.4 TFLite).
//
// The loader resolves model and label paths from the gallery directory or explicit
// config overrides in settings.BirdNETv3. It shares the ONNX Runtime path with
// the primary BirdNET settings since all ONNX models use the same runtime.
func (o *Orchestrator) loadBirdNETv3(threads int) error {
	log := GetLogger()

	modelPath := o.Settings.BirdNETv3.ModelPath
	labelPath := o.Settings.BirdNETv3.LabelPath

	// Fall back to gallery-installed paths if not explicitly configured.
	if modelPath == "" || labelPath == "" {
		m, l, _ := o.resolveInstalledPaths(RegistryIDBirdNETV3)
		if modelPath == "" {
			modelPath = m
		}
		if labelPath == "" {
			labelPath = l
		}
	}

	if modelPath == "" {
		return errors.Newf("BirdNET v3.0 model path not configured and model not installed via gallery").
			Component("classifier.orchestrator").
			Category(errors.CategoryModelInit).
			Context("model", RegistryIDBirdNETV3).
			Build()
	}

	// Pre-check ORT availability before attempting load.
	ortPath := o.Settings.BirdNET.ONNXRuntimePath
	ortStatus := inference.CheckORTAvailability(ortPath)
	if !ortStatus.Available {
		log.Warn("BirdNET v3.0 requires ONNX Runtime which is not available",
			logger.String("error", ortStatus.Error))
		emitORTUnavailableNotification("BirdNET v3.0", ortStatus.Error)
		return errors.Newf("BirdNET v3.0 requires ONNX Runtime %s: %s",
			inference.ORTRequiredVersion(), ortStatus.Error).
			Component("classifier.orchestrator").
			Category(errors.CategoryModelInit).
			Context("model", RegistryIDBirdNETV3).
			Context("ort_error", ortStatus.Error).
			Build()
	}

	// Build a settings clone with v3-specific overrides so the BirdNET
	// constructor uses the correct model path, labels, and threading.
	v3Settings := conf.CloneSettings(o.Settings)
	v3Settings.BirdNET.Version = "3.0"
	v3Settings.BirdNET.ModelPath = modelPath
	v3Settings.BirdNET.LabelPath = labelPath
	v3Settings.BirdNET.Threads = threads

	// Inherit locale from v3-specific config or fall back to primary.
	if o.Settings.BirdNETv3.Locale != "" {
		v3Settings.BirdNET.Locale = o.Settings.BirdNETv3.Locale
	}

	// Provide the resolved model info so NewBirdNET uses tier-1 resolution.
	v3Info := ModelRegistry[RegistryIDBirdNETV3]
	v3Info.CustomPath = modelPath

	bn, err := NewBirdNET(v3Settings, &v3Info)
	if err != nil {
		return errors.New(err).
			Component("classifier.orchestrator").
			Category(errors.CategoryModelInit).
			Context("model", RegistryIDBirdNETV3).
			Context("model_path", modelPath).
			Build()
	}

	o.models[bn.ModelInfo.ID] = &modelEntry{instance: bn}

	log.Info("BirdNET v3.0 model loaded into Orchestrator (side-by-side mode)",
		logger.String("model_id", bn.ModelInfo.ID),
		logger.Int("species", bn.NumSpecies()),
		logger.String("model_path", modelPath))

	return nil
}
