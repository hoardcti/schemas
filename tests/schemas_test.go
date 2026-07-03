package schemas

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// repoRoot is the repository root, relative to this test module.
const repoRoot = ".."

// schemaFilesEnv optionally restricts which schema files are tested. CI sets
// it to a comma-separated list of repository-relative paths (the schema
// files changed in a pull request); when unset or empty, every schema file
// in the repository is tested.
const schemaFilesEnv = "SCHEMA_FILES"

// TestSchemaStyle enforces the syntax and style rules in style.go for every
// schema file in the repository.
func TestSchemaStyle(t *testing.T) {
	for _, file := range findSchemaFiles(t) {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(repoRoot, file))
			if err != nil {
				t.Fatalf("reading schema: %v", err)
			}
			for _, problem := range checkStyle(file, raw) {
				t.Error(problem)
			}
		})
	}
}

// TestSchemaMetaValidation validates every schema file against the
// meta-schema declared in its $schema keyword, then walks up the meta-schema
// chain (parent, grandparent, ... up to maxMetaDepth levels) validating each
// level the same way. Remote meta-schemas are fetched over the network with
// caching, so the test is skipped in -short mode.
func TestSchemaMetaValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("fetches remote meta-schemas; skipped in -short mode")
	}
	validator := newMetaValidator()
	for _, file := range findSchemaFiles(t) {
		t.Run(file, func(t *testing.T) {
			doc := readSchemaDocument(t, filepath.Join(repoRoot, file))
			if err := validator.validateChain(doc, file); err != nil {
				t.Error(err)
			}
		})
	}
}

// findSchemaFiles returns the schema files to test as slash-separated paths
// relative to the repository root: the explicit list from SCHEMA_FILES when
// set, otherwise every schema file found by walking the repository. It fails
// the test if none are found, so a broken discovery step cannot pass
// silently.
func findSchemaFiles(t *testing.T) []string {
	t.Helper()
	if list := strings.TrimSpace(os.Getenv(schemaFilesEnv)); list != "" {
		return explicitSchemaFiles(t, list)
	}

	var files []string
	err := filepath.WalkDir(repoRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip VCS internals and this test module itself.
			if d.Name() == ".git" || d.Name() == "tests" {
				return filepath.SkipDir
			}
			return nil
		}
		if isSchemaFile(d.Name()) {
			rel, err := filepath.Rel(repoRoot, p)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repository: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no *.schema.json files found in the repository")
	}
	return files
}

// explicitSchemaFiles parses the comma-separated SCHEMA_FILES list and
// verifies every entry names an existing schema file, so a typo in the CI
// wiring fails the build instead of silently testing nothing.
func explicitSchemaFiles(t *testing.T, list string) []string {
	t.Helper()
	var files []string
	for _, entry := range strings.Split(list, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !isSchemaFile(path.Base(entry)) {
			t.Fatalf("%s entry %q is not a schema file", schemaFilesEnv, entry)
		}
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(entry))); err != nil {
			t.Fatalf("%s entry %q: %v", schemaFilesEnv, entry, err)
		}
		files = append(files, entry)
	}
	if len(files) == 0 {
		t.Fatalf("%s is set but names no schema files", schemaFilesEnv)
	}
	return files
}

// readSchemaDocument decodes a schema file into the generic representation
// the jsonschema library validates against.
func readSchemaDocument(t *testing.T, path string) any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening schema: %v", err)
	}
	defer f.Close()

	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("decoding schema: %v", err)
	}
	return doc
}
