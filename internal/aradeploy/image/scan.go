package image

import (
	"io/fs"
	"regexp"
	"strings"
)

// Entry represents any image found in a template.
type Entry struct {
	AppName  string
	FilePath string
	Image    string
	Ref      Ref
}

// FloatingTagEntry represents a floating tag found in a template.
type FloatingTagEntry struct {
	AppName  string
	FilePath string
	Image    string
	Ref      Ref
}

// imageLineRe matches YAML "image:" keys with proper indentation.
var imageLineRe = regexp.MustCompile(`(?m)^\s+image:\s*(.+?)(?:\s*#.*)?$`)
var templateRe = regexp.MustCompile(`\{\{`)

// scanImages is the shared implementation for scanning image references from template files.
func scanImages(tmplFS fs.FS, floatingOnly bool) ([]Entry, error) {
	var entries []Entry

	err := fs.WalkDir(tmplFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		if !strings.HasSuffix(path, "docker-compose.yml.tmpl") {
			return nil
		}

		data, err := fs.ReadFile(tmplFS, path)
		if err != nil {
			return err
		}

		matches := imageLineRe.FindAllStringSubmatch(string(data), -1)
		for _, m := range matches {
			imageStr := strings.TrimSpace(m[1])

			if templateRe.MatchString(imageStr) {
				continue
			}

			ref, err := ParseRef(imageStr)
			if err != nil {
				continue
			}

			if floatingOnly && !ref.IsFloating() {
				continue
			}

			parts := strings.SplitN(path, "/", 2)
			appName := parts[0]
			entries = append(entries, Entry{
				AppName:  appName,
				FilePath: path,
				Image:    imageStr,
				Ref:      ref,
			})
		}

		return nil
	})

	return entries, err
}

// ScanAll scans all templates for image references, returning all images.
func ScanAll(tmplFS fs.FS) ([]Entry, error) {
	return scanImages(tmplFS, false)
}

// ScanDeployed parses image references from a deployed docker-compose.yml file.
func ScanDeployed(data []byte) ([]Ref, error) {
	matches := imageLineRe.FindAllStringSubmatch(string(data), -1)

	var refs []Ref
	for _, m := range matches {
		imageStr := strings.TrimSpace(m[1])
		ref, err := ParseRef(imageStr)
		if err != nil {
			continue
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// ScanFloatingTags scans all templates for floating image tags.
func ScanFloatingTags(tmplFS fs.FS) ([]FloatingTagEntry, error) {
	images, err := scanImages(tmplFS, true)
	if err != nil {
		return nil, err
	}
	entries := make([]FloatingTagEntry, len(images))
	for i, img := range images {
		entries[i] = FloatingTagEntry(img)
	}
	return entries, nil
}
