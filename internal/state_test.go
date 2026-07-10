package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFile(t *testing.T) {
	levels, err := Load("/nonexistent/path.state")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
	if levels.Pump != 0 || levels.Fan != 0 {
		t.Errorf("expected {0,0}, got {%d,%d}", levels.Pump, levels.Fan)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.state")
	os.WriteFile(path, []byte{}, 0644)

	levels, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if levels.Pump != 0 || levels.Fan != 0 {
		t.Errorf("expected {0,0}, got {%d,%d}", levels.Pump, levels.Fan)
	}
}

func TestLoad_CorruptData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.state")
	os.WriteFile(path, []byte("hello world\n"), 0644)

	levels, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if levels.Pump != 0 || levels.Fan != 0 {
		t.Errorf("expected {0,0}, got {%d,%d}", levels.Pump, levels.Fan)
	}
}

func TestLoad_OutOfRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outofrange.state")
	os.WriteFile(path, []byte("42 99\n"), 0644)

	levels, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if levels.Pump != 0 || levels.Fan != 0 {
		t.Errorf("expected {0,0} for out-of-range values, got {%d,%d}", levels.Pump, levels.Fan)
	}
}

func TestLoad_NegativeValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "negative.state")
	os.WriteFile(path, []byte("-1 -1\n"), 0644)

	levels, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if levels.Pump != 0 || levels.Fan != 0 {
		t.Errorf("expected {0,0} for negative values, got {%d,%d}", levels.Pump, levels.Fan)
	}
}

func TestLoad_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "valid.state")

	Save(path, Levels{Pump: 2, Fan: 3})

	levels, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if levels.Pump != 2 || levels.Fan != 3 {
		t.Errorf("expected {2,3}, got {%d,%d}", levels.Pump, levels.Fan)
	}
}

func TestLoad_ValidEmergency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "emergency.state")

	Save(path, Levels{Pump: 4, Fan: 4})

	levels, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if levels.Pump != 4 || levels.Fan != 4 {
		t.Errorf("expected {4,4}, got {%d,%d}", levels.Pump, levels.Fan)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.state")

	original := Levels{Pump: 1, Fan: 2}
	if err := Save(path, original); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded != original {
		t.Errorf("roundtrip: expected %+v, got %+v", original, loaded)
	}
}

func TestSave_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "idempotent.state")

	Save(path, Levels{Pump: 0, Fan: 0})

	first, _ := Load(path)
	second, _ := Load(path)
	if first != second {
		t.Errorf("idempotent: expected same result, got %+v vs %+v", first, second)
	}
}
