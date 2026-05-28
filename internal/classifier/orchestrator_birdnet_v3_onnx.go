package classifier

import (
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/errors"
	"github.com/tphakala/birdnet-go/internal/logger"
)

// loadBirdNETv3 creates and registers a BirdNET v3.0 ONNX model instance as a
// secondary model for side-by-side comparison with the primary v2.4.
func (o *Orchestrator) loadBirdNETv3(threads int) error {
	log := GetLogger()

	modelPath := o.Settings.BirdNETv3.ModelPath
	labelPath := o.Settings.BirdNETv3.LabelPath

	if modelPath == "" || labelPath == "" {
		m, l, _ := o.resolveInstalledPaths(RegistryIDBirdNETV3)
		if modelPath == "" {
			modelPath = m
		}
		if labelPath == "" {
			labelPath = l
		}
	}

	if modelPath == "" || labelPath == "" {
		return errors.Newf("BirdNET v3.0 model files not installed or configured").
			Component("classifier.orchestrator").
			Category(errors.CategoryModelInit).
			Context("model", "BirdNET_V3.0").
			Build()
	}

	if err := checkORTOrFail(o.Settings.BirdNET.ONNXRuntimePath, "BirdNET v3.0", "BirdNET_V3.0", "classifier.orchestrator"); err != nil {
		return err
	}

	v3Settings := conf.CloneSettings(o.Settings)
	v3Settings.BirdNET.Version = "3.0"
	v3Settings.BirdNET.ModelPath = modelPath
	v3Settings.BirdNET.LabelPath = labelPath
	v3Settings.BirdNET.Threads = threads

	if o.Settings.BirdNETv3.Locale != "" {
		v3Settings.BirdNET.Locale = o.Settings.BirdNETv3.Locale
	}

	v3Info := ModelRegistry[RegistryIDBirdNETV3]

	bn, err := NewBirdNET(v3Settings, &v3Info)
	if err != nil {
		return errors.New(err).
			Component("classifier.orchestrator").
			Category(errors.CategoryModelInit).
			Context("model", "BirdNET_V3.0").
			Context("model_path", modelPath).
			Build()
	}

	o.models[bn.ModelID()] = &modelEntry{instance: bn}

	log.Info("BirdNET v3.0 model loaded into Orchestrator",
		logger.String("model_id", bn.ModelID()),
		logger.Int("species", bn.NumSpecies()))

	return nil
}
