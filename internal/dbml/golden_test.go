package dbml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGolden parses every testdata/*.dbml, prints it and compares the result
// with the .golden.dbml next to it. Set MESA_UPDATE_GOLDEN=1 to rewrite the
// expected files after an intended change to the printer.
func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob(filepath.Join("testdata", "*.dbml"))
	if err != nil {
		t.Fatal(err)
	}
	update := os.Getenv("MESA_UPDATE_GOLDEN") != ""
	ran := 0
	for _, in := range inputs {
		if strings.HasSuffix(in, ".golden.dbml") {
			continue
		}
		ran++
		name := strings.TrimSuffix(filepath.Base(in), ".dbml")
		golden := filepath.Join("testdata", name+".golden.dbml")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			db, err := Parse(string(src))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got := Print(db)
			if update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run with MESA_UPDATE_GOLDEN=1 to create it)", err)
			}
			if got != string(want) {
				t.Errorf("Print(Parse(%s)) differs from %s:\n--- got ---\n%s\n--- want ---\n%s", in, golden, got, want)
			}

			// The printed form is a fixed point: parsing and printing it
			// again changes nothing.
			again, err := Parse(got)
			if err != nil {
				t.Fatalf("Parse(Print(…)): %v", err)
			}
			if second := Print(again); second != got {
				t.Errorf("Print is not idempotent:\n--- first ---\n%s\n--- second ---\n%s", got, second)
			}
			if problems := Validate(again); len(problems) != len(Validate(db)) {
				t.Errorf("Validate differs after a round trip: %v vs %v", problems, Validate(db))
			}
		})
	}
	if ran == 0 {
		t.Fatal("no testdata/*.dbml inputs found")
	}
}
