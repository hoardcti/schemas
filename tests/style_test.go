package schemas

import (
	"strings"
	"testing"
)

// validSchema is a minimal document that satisfies every style rule; the
// rejection cases below each break exactly one rule.
const validSchema = `{
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "$id": "https://schema.hoardcti.com/v1.0.0/example.schema.json",
    "title": "Example",
    "description": "Example schema used by the style unit tests.",
    "type": "object"
}
`

func TestCheckStyleAcceptsCleanFile(t *testing.T) {
	if problems := checkStyle("v1.0.0/example.schema.json", []byte(validSchema)); len(problems) > 0 {
		t.Errorf("expected no problems, got: %v", problems)
	}
}

func TestCheckStyleRejects(t *testing.T) {
	cases := []struct {
		name    string
		relPath string
		content string
		want    string // substring expected in one reported problem
	}{
		{
			name:    "invalid JSON",
			relPath: "v1.0.0/example.schema.json",
			content: "{ not json",
			want:    "invalid JSON",
		},
		{
			name:    "duplicate key",
			relPath: "v1.0.0/example.schema.json",
			content: strings.Replace(validSchema, `"title": "Example",`, `"title": "Example",`+"\n"+`    "title": "Example",`, 1),
			want:    "duplicate object key",
		},
		{
			name:    "missing required keyword",
			relPath: "v1.0.0/example.schema.json",
			content: strings.Replace(validSchema, `    "description": "Example schema used by the style unit tests.",`+"\n", "", 1),
			want:    `missing required top-level keyword "description"`,
		},
		{
			name:    "tab indentation",
			relPath: "v1.0.0/example.schema.json",
			content: strings.Replace(validSchema, `    "title"`, "\t\"title\"", 1),
			want:    "indent with spaces",
		},
		{
			name:    "non-https $id",
			relPath: "v1.0.0/example.schema.json",
			content: strings.Replace(validSchema, "https://schema.hoardcti.com", "http://schema.hoardcti.com", 1),
			want:    "absolute https URL",
		},
		{
			name:    "$id does not match file name",
			relPath: "v1.0.0/other.schema.json",
			content: validSchema,
			want:    "must end with",
		},
		{
			name:    "missing trailing newline",
			relPath: "v1.0.0/example.schema.json",
			content: strings.TrimSuffix(validSchema, "\n"),
			want:    "end with a newline",
		},
		{
			name:    "uppercase file name",
			relPath: "v1.0.0/Example.schema.json",
			content: strings.Replace(validSchema, "example.schema.json", "Example.schema.json", 1),
			want:    "must be lowercase",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := checkStyle(tc.relPath, []byte(tc.content))
			for _, p := range problems {
				if strings.Contains(p, tc.want) {
					return
				}
			}
			t.Errorf("expected a problem containing %q, got: %v", tc.want, problems)
		})
	}
}
