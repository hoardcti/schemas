package schemas

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"
)

// requiredTopLevelKeywords are the keywords every published schema must
// declare so consumers can identify, resolve and document it.
var requiredTopLevelKeywords = []string{"$schema", "$id", "title", "description", "type"}

// isSchemaFile reports whether a file name follows the schema naming
// convention used across HoardCTI repositories: <name>.schema.json, or a
// bare schema.json.
func isSchemaFile(name string) bool {
	return name == "schema.json" || strings.HasSuffix(name, ".schema.json")
}

// checkStyle applies every syntax and style rule to one schema file and
// returns one message per violation. An empty result means the file is
// clean. relPath is the file's path relative to the repository root (forward
// slashes); raw is its exact on-disk content.
func checkStyle(relPath string, raw []byte) []string {
	var problems []string
	problems = append(problems, checkEncoding(raw)...)
	problems = append(problems, checkLayout(raw)...)
	problems = append(problems, checkNaming(relPath)...)
	problems = append(problems, checkDocument(relPath, raw)...)
	return problems
}

// checkEncoding enforces that the file is plain UTF-8 without a byte order
// mark, which some JSON parsers reject.
func checkEncoding(raw []byte) []string {
	var problems []string
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		problems = append(problems, "file starts with a UTF-8 byte order mark; remove it")
	}
	if !utf8.Valid(raw) {
		problems = append(problems, "file is not valid UTF-8")
	}
	return problems
}

// checkLayout enforces whitespace conventions: spaces for indentation, no
// trailing whitespace, and a newline at end of file.
func checkLayout(raw []byte) []string {
	var problems []string
	text := normalizeNewlines(raw)
	if !bytes.HasSuffix(text, []byte("\n")) {
		problems = append(problems, "file must end with a newline")
	}
	for i, line := range strings.Split(strings.TrimSuffix(string(text), "\n"), "\n") {
		if strings.Contains(line, "\t") {
			problems = append(problems, fmt.Sprintf("line %d: indent with spaces, not tabs", i+1))
		}
		if line != strings.TrimRight(line, " ") {
			problems = append(problems, fmt.Sprintf("line %d: trailing whitespace", i+1))
		}
	}
	return problems
}

// checkNaming enforces lowercase file names so published schema URLs are
// case-stable across hosts.
func checkNaming(relPath string) []string {
	if name := path.Base(relPath); name != strings.ToLower(name) {
		return []string{fmt.Sprintf("file name %q must be lowercase", name)}
	}
	return nil
}

// checkDocument parses the file and enforces rules on the schema document
// itself: well-formed JSON with no duplicate keys, the required top-level
// keywords, and $schema / $id values that are absolute URLs consistent with
// the file's location in the repository.
func checkDocument(relPath string, raw []byte) []string {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		// Nothing below is meaningful when the file does not parse.
		return []string{fmt.Sprintf("invalid JSON: %v", err)}
	}

	var problems []string

	dups, err := duplicateKeys(raw)
	if err != nil {
		problems = append(problems, fmt.Sprintf("duplicate-key scan failed: %v", err))
	}
	for _, dup := range dups {
		problems = append(problems, fmt.Sprintf("duplicate object key at %s", dup))
	}

	for _, key := range requiredTopLevelKeywords {
		if _, ok := doc[key]; !ok {
			problems = append(problems, fmt.Sprintf("missing required top-level keyword %q", key))
		}
	}
	problems = append(problems, checkURLKeyword(doc, "$schema")...)
	problems = append(problems, checkURLKeyword(doc, "$id")...)

	// The published $id must point at this file, so the URL a consumer
	// resolves matches the repository layout.
	if id, ok := doc["$id"].(string); ok {
		if want := "/" + path.Base(relPath); !strings.HasSuffix(id, want) {
			problems = append(problems, fmt.Sprintf("$id %q must end with %q to match the file name", id, want))
		}
	}
	return problems
}

// checkURLKeyword verifies that the named keyword, when present, is an
// absolute https URL. Absence is reported by the required-keyword check.
func checkURLKeyword(doc map[string]any, key string) []string {
	raw, ok := doc[key]
	if !ok {
		return nil
	}
	s, ok := raw.(string)
	if !ok {
		return []string{fmt.Sprintf("%s must be a string", key)}
	}
	u, err := url.Parse(s)
	if err != nil || !u.IsAbs() || u.Scheme != "https" {
		return []string{fmt.Sprintf("%s %q must be an absolute https URL", key, s)}
	}
	return nil
}

// duplicateKeys walks the raw JSON token stream and returns the path of
// every object key that appears more than once. encoding/json silently keeps
// the last value for a duplicated key, so scanning tokens is the only way to
// catch this class of mistake.
func duplicateKeys(raw []byte) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	return scanValue(dec, "$")
}

// scanValue consumes one JSON value from dec, recursing into objects and
// arrays. path is the JSONPath-style location of the value, used in reports.
func scanValue(dec *json.Decoder, path string) ([]string, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil, nil // scalar: nothing to recurse into
	}

	var dups []string
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key := keyTok.(string) // object keys are always strings
			if seen[key] {
				dups = append(dups, path+"."+key)
			}
			seen[key] = true

			nested, err := scanValue(dec, path+"."+key)
			if err != nil {
				return nil, err
			}
			dups = append(dups, nested...)
		}
	case '[':
		for i := 0; dec.More(); i++ {
			nested, err := scanValue(dec, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			dups = append(dups, nested...)
		}
	}
	// Consume the closing '}' or ']'.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return dups, nil
}

// normalizeNewlines converts CRLF line endings to LF so layout checks behave
// identically regardless of the git checkout settings on the host.
func normalizeNewlines(raw []byte) []byte {
	return bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
}
