package template

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

// Renderer renders Go templates from app templates.
type Renderer struct {
	registry *Registry
}

// NewRenderer creates a new renderer backed by the registry.
func NewRenderer(registry *Registry) *Renderer {
	return &Renderer{registry: registry}
}

// RenderFile renders a single template file for the given app with provided values.
func (r *Renderer) RenderFile(appName, fileName string, values map[string]string) (string, error) {
	tmplPath := appName + "/" + fileName
	data, err := fs.ReadFile(r.registry.FS(), tmplPath)
	if err != nil {
		return "", fmt.Errorf("reading template %s: %w", tmplPath, err)
	}

	return r.renderString(string(data), values)
}

// RenderAllFiles renders all .tmpl files for an app and returns filename->content map.
func (r *Renderer) RenderAllFiles(appName string, values map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	tmplDir := appName

	err := fs.WalkDir(r.registry.FS(), tmplDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		if !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		data, err := fs.ReadFile(r.registry.FS(), path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		rendered, err := r.renderString(string(data), values)
		if err != nil {
			return fmt.Errorf("rendering %s: %w", path, err)
		}

		relPath := strings.TrimPrefix(path, tmplDir+"/")
		outName := strings.TrimSuffix(relPath, ".tmpl")
		result[outName] = rendered
		return nil
	})

	return result, err
}

// CopyStaticFiles returns non-template files that should be copied as-is.
func (r *Renderer) CopyStaticFiles(appName string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	tmplDir := appName

	err := fs.WalkDir(r.registry.FS(), tmplDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		if strings.HasSuffix(path, ".tmpl") || strings.HasSuffix(path, "app.yaml") {
			return nil
		}

		data, err := fs.ReadFile(r.registry.FS(), path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		relPath := strings.TrimPrefix(path, tmplDir+"/")
		result[relPath] = data
		return nil
	})

	return result, err
}

func (r *Renderer) renderString(tmplStr string, values map[string]string) (string, error) {
	funcMap := template.FuncMap{
		"default": func(def, val string) string {
			if val == "" {
				return def
			}
			return val
		},
		"genPassword": GenPassword,
		"genUUID":     func() string { return uuid.New().String() },
		"upper":       strings.ToUpper,
		"lower":       strings.ToLower,
		"replace":     strings.ReplaceAll,
		"argon2Hash":  Argon2Hash,
	}

	tmpl, err := template.New("").Option("missingkey=error").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, values); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

// GenPassword generates a random hex password.
func GenPassword() (string, error) {
	return genRandomHex(16)
}

func genRandomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand failed: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Argon2Hash produces an argon2id hash string compatible with Authelia's user database format.
func Argon2Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	// Authelia defaults: memory=65536 KiB, iterations=3, parallelism=4, keyLen=32
	const (
		memory      = 65536
		iterations  = 3
		parallelism = 4
		keyLen      = 32
	)

	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLen)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory, iterations, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}
