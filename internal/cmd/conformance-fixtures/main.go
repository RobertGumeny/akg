package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RobertGumeny/akg-format/internal/store"
)

func main() {
	dir := flag.String("dir", "testdata/conformance", "conformance fixture directory")
	printHashes := flag.Bool("print-hashes", false, "print current fixture sha256 values")
	flag.Parse()

	manifest, err := loadManifest(filepath.Join(*dir, "manifest.json"))
	if err != nil {
		fatal(err)
	}
	for _, fixture := range manifest.Fixtures {
		path := filepath.Join(*dir, fixture.Path)
		data, err := os.ReadFile(path)
		if err != nil {
			fatal(fmt.Errorf("read %s: %w", fixture.Path, err))
		}
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		if *printHashes {
			fmt.Printf("%s  %s\n", hash, fixture.Path)
		}
		if fixture.SHA256 == "" {
			fatal(fmt.Errorf("%s: manifest sha256 is empty", fixture.Path))
		}
		if hash != fixture.SHA256 {
			fatal(fmt.Errorf("%s: sha256 %s, want %s", fixture.Path, hash, fixture.SHA256))
		}
		if fixture.ValidationScope == "store" {
			err := store.Validate(path)
			if fixture.ExpectedResult == "accept" && err != nil {
				fatal(fmt.Errorf("%s: validate rejected accepted fixture: %w", fixture.Path, err))
			}
			if fixture.ExpectedResult == "reject" && err == nil {
				fatal(fmt.Errorf("%s: validate accepted rejection fixture", fixture.Path))
			}
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func loadManifest(path string) (manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, err
	}
	if m.Version != 1 {
		return manifest{}, fmt.Errorf("manifest version = %d, want 1", m.Version)
	}
	return m, nil
}

type manifest struct {
	Version  int       `json:"version"`
	Fixtures []fixture `json:"fixtures"`
}

type fixture struct {
	Path            string `json:"path"`
	ExpectedResult  string `json:"expected_result"`
	ValidationScope string `json:"validation_scope"`
	SHA256          string `json:"sha256"`
}
