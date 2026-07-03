// Package seeddetections provides the hidden `birdnet-go seed-detections`
// subcommand: it writes a set of synthetic bird detections spanning several days
// into a fresh SQLite database using the real v2 datastore write path. This gives
// frontend/UI work (e.g. the analytics Species and Activity pages) realistic data
// to render and click through without needing a live audio source or the model.
//
// It is a development/test aid, not a production command, so it is Hidden. It
// refuses to overwrite an existing database.
package seeddetections

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/tphakala/birdnet-go/internal/classifier"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/datastore"
	"github.com/tphakala/birdnet-go/internal/datastore/v2only"
	"github.com/tphakala/birdnet-go/internal/detection"
	"github.com/tphakala/birdnet-go/internal/logger"
)

// seedSpecies is one synthetic species and the per-day detection pattern used to
// generate its notes. Times/counts are chosen so the analytics pages have varied,
// realistic-looking data: different day counts, hours, and confidences.
type seedSpecies struct {
	scientificName string
	commonName     string
	// hours the species is "detected" at each day; repeated hours yield multiple
	// detections in that hour so the time-of-day and hourly views are non-trivial.
	hours []int
	// baseConfidence is nudged per-detection so avg/max confidence differ per species.
	baseConfidence float64
}

var seedCatalog = []seedSpecies{
	{"Turdus merula", "Eurasian Blackbird", []int{5, 6, 6, 7, 18, 19}, 0.82},
	{"Erithacus rubecula", "European Robin", []int{6, 7, 7, 8, 17}, 0.74},
	{"Cyanistes caeruleus", "Eurasian Blue Tit", []int{8, 9, 12, 14}, 0.68},
	{"Fringilla coelebs", "Common Chaffinch", []int{7, 10, 11, 15, 16}, 0.79},
	{"Parus major", "Great Tit", []int{6, 8, 9, 13, 17, 18}, 0.71},
	{"Phoenicurus phoenicurus", "Common Redstart", []int{5, 6, 20}, 0.66},
}

// Command builds the hidden seed-detections subcommand.
func Command(settings *conf.Settings) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:    "seed-detections",
		Short:  "Seed a fresh SQLite database with synthetic detections for UI/frontend testing (internal)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSeed(settings, days)
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "number of days back from today to spread detections over")
	return cmd
}

func runSeed(settings *conf.Settings, days int) error {
	if days < 1 {
		return fmt.Errorf("--days must be >= 1")
	}

	// Force SQLite; seeding targets the configured sqlite path.
	settings.Output.SQLite.Enabled = true
	settings.Output.MySQL.Enabled = false
	dbPath := settings.Output.SQLite.Path
	if dbPath == "" {
		return fmt.Errorf("output.sqlite.path is empty; set it in config or pass a config with a path")
	}
	if _, err := os.Stat(dbPath); err == nil {
		return fmt.Errorf("database %q already exists; seed-detections only creates a fresh DB (move it aside first)", dbPath)
	}

	log := logger.Global().Module("seed-detections")

	// Load taxonomy so the fresh v2 datastore can resolve species codes, matching
	// what the app does on a fresh install.
	_, sciIndex, err := classifier.LoadTaxonomyData("")
	if err != nil {
		return fmt.Errorf("load taxonomy: %w", err)
	}

	ds, err := v2only.InitializeFreshInstall(settings, log, sciIndex)
	if err != nil {
		return fmt.Errorf("initialize fresh install: %w", err)
	}
	defer func() { _ = ds.Close() }()

	modelInfo := detection.DefaultModelInfo()
	if err := ds.EnsureModelRegistered(modelInfo); err != nil {
		return fmt.Errorf("register model: %w", err)
	}

	// Seed is deterministic (no randomness): dates walk back from today, and the
	// per-species hour lists drive counts, so re-seeding a fresh DB is reproducible.
	today := time.Now()
	total := 0
	for d := 0; d < days; d++ {
		day := today.AddDate(0, 0, -d)
		dateStr := day.Format("2006-01-02")
		for si := range seedCatalog {
			s := seedCatalog[si]
			// Skip some species on some days so first/last-heard and counts vary.
			if (d+si)%4 == 3 {
				continue
			}
			for hi, hour := range s.hours {
				conf := s.baseConfidence + float64(hi)*0.01
				if conf > 0.99 {
					conf = 0.99
				}
				begin := time.Date(day.Year(), day.Month(), day.Day(), hour, (hi*7)%60, 0, 0, day.Location())
				note := &datastore.Note{
					SourceNode:     "seed",
					Date:           dateStr,
					Time:           begin.Format("15:04:05"),
					BeginTime:      begin,
					EndTime:        begin.Add(3 * time.Second),
					ScientificName: s.scientificName,
					CommonName:     s.commonName,
					Confidence:     conf,
					Model:          modelInfo,
				}
				results := []datastore.Results{{Species: s.scientificName, Confidence: float32(conf)}}
				if err := ds.Save(note, results); err != nil {
					return fmt.Errorf("save note (%s %s): %w", dateStr, s.scientificName, err)
				}
				total++
			}
		}
	}

	fmt.Printf("Seeded %d detections across %d days into %s\n", total, days, dbPath)
	return nil
}
