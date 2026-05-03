//go:build onnx

package onnx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLabelsText(t *testing.T) {
	data := []byte("Parus major_Great Tit\nTurdus merula_Common Blackbird\n\n  Erithacus rubecula_European Robin  \n")
	labels, err := loadLabelsText(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 3 {
		t.Fatalf("got %d labels, want 3", len(labels))
	}
	if labels[0] != "Parus major_Great Tit" {
		t.Errorf("labels[0] = %q", labels[0])
	}
	if labels[2] != "Erithacus rubecula_European Robin" {
		t.Errorf("labels[2] = %q, want trimmed", labels[2])
	}
}

func TestLoadLabelsCSV(t *testing.T) {
	t.Run("comma_sci_name", func(t *testing.T) {
		data := []byte("id,sci_name,com_name\n1,Parus major,Great Tit\n2,Turdus merula,Common Blackbird\n")
		labels, err := loadLabelsCSV(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(labels) != 2 {
			t.Fatalf("got %d labels, want 2", len(labels))
		}
		if labels[0] != "Parus major" {
			t.Errorf("labels[0] = %q, want sci_name column", labels[0])
		}
	})

	t.Run("semicolon_delimiter", func(t *testing.T) {
		data := []byte("id;sci_name;com_name\n1;Parus major;Great Tit\n")
		labels, err := loadLabelsCSV(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(labels) != 1 || labels[0] != "Parus major" {
			t.Errorf("got %v", labels)
		}
	})

	t.Run("com_name_fallback", func(t *testing.T) {
		data := []byte("id,com_name\n1,Great Tit\n2,Blackbird\n")
		labels, err := loadLabelsCSV(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(labels) != 2 || labels[0] != "Great Tit" {
			t.Errorf("got %v", labels)
		}
	})

	t.Run("no_recognized_column", func(t *testing.T) {
		data := []byte("id,species_code\n1,parmaj\n")
		_, err := loadLabelsCSV(data)
		if err == nil {
			t.Error("expected error for unrecognized column")
		}
	})

	t.Run("too_few_rows", func(t *testing.T) {
		data := []byte("sci_name\n")
		_, err := loadLabelsCSV(data)
		if err == nil {
			t.Error("expected error for CSV with only header")
		}
	})
}

func TestLoadLabelsJSON(t *testing.T) {
	t.Run("string_array", func(t *testing.T) {
		data := []byte(`["Parus major","Turdus merula"]`)
		labels, err := loadLabelsJSON(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(labels) != 2 || labels[0] != "Parus major" {
			t.Errorf("got %v", labels)
		}
	})

	t.Run("labels_object", func(t *testing.T) {
		data := []byte(`{"labels":["A","B","C"]}`)
		labels, err := loadLabelsJSON(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(labels) != 3 {
			t.Errorf("got %d labels", len(labels))
		}
	})

	t.Run("named_objects", func(t *testing.T) {
		data := []byte(`[{"name":"A"},{"name":"B"}]`)
		labels, err := loadLabelsJSON(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(labels) != 2 || labels[0] != "A" {
			t.Errorf("got %v", labels)
		}
	})

	t.Run("unrecognized_format", func(t *testing.T) {
		data := []byte(`{"species":["A"]}`)
		_, err := loadLabelsJSON(data)
		if err == nil {
			t.Error("expected error for unrecognized JSON format")
		}
	})
}

func TestLoadLabelsFromBytes_Extension(t *testing.T) {
	_, err := loadLabelsFromBytes([]byte("data"), ".xml")
	if err == nil {
		t.Error("expected error for unsupported extension")
	}
}

func TestLoadLabels_FileIntegration(t *testing.T) {
	dir := t.TempDir()

	t.Run("text_file", func(t *testing.T) {
		path := filepath.Join(dir, "labels.txt")
		if err := os.WriteFile(path, []byte("Species A\nSpecies B\n"), 0644); err != nil {
			t.Fatal(err)
		}
		labels, err := loadLabels(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(labels) != 2 {
			t.Errorf("got %d labels", len(labels))
		}
	})

	t.Run("missing_file", func(t *testing.T) {
		_, err := loadLabels(filepath.Join(dir, "nonexistent.txt"))
		if err == nil {
			t.Error("expected error for missing file")
		}
		var le *LabelLoadError
		if _, ok := err.(*LabelLoadError); !ok {
			t.Errorf("expected LabelLoadError, got %T", le)
		}
	})
}
