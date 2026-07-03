package schemas

import (
	"fmt"
	"net/url"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// maxMetaDepth bounds how many levels of the meta-schema chain are validated
// for a single file: the schema itself, its parent, grandparent, and so on.
const maxMetaDepth = 5

// metaValidator validates schema documents against their meta-schema chain.
// One instance is shared by all test cases so compiled schemas and fetched
// documents are reused across files.
type metaValidator struct {
	compiler *jsonschema.Compiler
	loader   *cachingLoader
}

func newMetaValidator() *metaValidator {
	loader := newCachingLoader()
	compiler := jsonschema.NewCompiler()
	// Route every remote reference the compiler resolves through the same
	// caching loader used to walk the chain, so nothing is fetched twice.
	compiler.UseLoader(jsonschema.SchemeURLLoader{
		"http":  loader,
		"https": loader,
	})
	return &metaValidator{compiler: compiler, loader: loader}
}

// validateChain checks doc against the meta-schema named in its $schema
// keyword, then repeats the check for the meta-schema itself, walking up the
// chain until it becomes self-referential or maxMetaDepth levels have been
// validated. The official drafts declare themselves as their own
// meta-schema, so the chain terminates naturally for them.
func (v *metaValidator) validateChain(doc any, docURL string) error {
	current, currentURL := doc, docURL
	for depth := 0; depth < maxMetaDepth; depth++ {
		metaURL, err := declaredMetaSchema(current)
		if err != nil {
			return fmt.Errorf("%s: %w", currentURL, err)
		}
		meta, err := v.compiler.Compile(metaURL)
		if err != nil {
			return fmt.Errorf("compile meta-schema %s: %w", metaURL, err)
		}
		if err := meta.Validate(current); err != nil {
			return fmt.Errorf("%s does not conform to %s: %w", currentURL, metaURL, err)
		}
		// A schema that is its own meta-schema ends the chain: validating
		// it again would repeat the check that just passed.
		if metaURL == currentURL {
			return nil
		}
		next, err := v.loader.Load(metaURL)
		if err != nil {
			return fmt.Errorf("fetch meta-schema %s: %w", metaURL, err)
		}
		current, currentURL = next, metaURL
	}
	return nil
}

// declaredMetaSchema returns the absolute URL held in the document's $schema
// keyword.
func declaredMetaSchema(doc any) (string, error) {
	obj, ok := doc.(map[string]any)
	if !ok {
		return "", fmt.Errorf("schema document is not a JSON object")
	}
	raw, ok := obj["$schema"]
	if !ok {
		return "", fmt.Errorf("missing $schema keyword")
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("$schema is not a string")
	}
	if u, err := url.Parse(s); err != nil || !u.IsAbs() {
		return "", fmt.Errorf("$schema %q is not an absolute URL", s)
	}
	return s, nil
}
