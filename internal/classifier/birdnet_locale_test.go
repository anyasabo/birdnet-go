package classifier

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tphakala/birdnet-go/internal/conf"
)

func TestNormalizeAndValidateLocale_UnsupportedFallsBack(t *testing.T) {
	t.Parallel()

	bn := &BirdNET{
		Settings: &conf.Settings{
			BirdNET: conf.BirdNETConfig{
				Locale: "en",
			},
		},
		ModelInfo: ModelInfo{
			ID:               "BirdNET_V2.4",
			DefaultLocale:    "en-uk",
			SupportedLocales: []string{"en-uk", "fr"},
		},
	}

	bn.normalizeAndValidateLocale()

	assert.Equal(t, "en-uk", bn.Settings.BirdNET.Locale)
}

func TestNormalizeAndValidateLocale_UnsupportedByModelUsesModelDefault(t *testing.T) {
	t.Parallel()

	bn := &BirdNET{
		Settings: &conf.Settings{
			BirdNET: conf.BirdNETConfig{
				Locale: "de",
			},
		},
		ModelInfo: ModelInfo{
			ID:               "BirdNET_V2.4",
			DefaultLocale:    "fr",
			SupportedLocales: []string{"fr"},
		},
	}

	bn.normalizeAndValidateLocale()

	assert.Equal(t, "fr", bn.Settings.BirdNET.Locale)
}
