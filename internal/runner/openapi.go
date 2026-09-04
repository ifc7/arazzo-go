package runner

import (
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"path/filepath"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// buildOpenAPIDocument parses an OpenAPI source and resolves $refs relative to sourceURL.
// Local file sources get a BasePath so sibling/parent refs like ../openapi.yaml work.
func buildOpenAPIDocument(sourceURL string, b []byte, logger *slog.Logger) (*v3.Document, error) {
	cfg := datamodel.NewDocumentConfiguration()
	if logger != nil {
		cfg.Logger = logger
	} else {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}

	if localPath, ok := localSourcePath(sourceURL); ok {
		cfg.BasePath = filepath.Dir(localPath)
		cfg.SpecFilePath = localPath
		cfg.AllowFileReferences = true
	} else if remote, err := url.Parse(sourceURL); err == nil && (remote.Scheme == "http" || remote.Scheme == "https") {
		cfg.AllowRemoteReferences = true
		dir := *remote
		dir.Path = path.Dir(remote.Path)
		if dir.Path == "." {
			dir.Path = "/"
		}
		cfg.BaseURL = &dir
	}

	document, err := libopenapi.NewDocumentWithConfiguration(b, cfg)
	if err != nil {
		return nil, fmt.Errorf("parse openapi %s: %w", sourceURL, err)
	}
	model, err := document.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("build openapi model %s: %w", sourceURL, err)
	}
	return &model.Model, nil
}

func localSourcePath(sourceURL string) (string, bool) {
	if sourceURL == "" {
		return "", false
	}
	u, err := url.Parse(sourceURL)
	if err != nil {
		return "", false
	}
	switch u.Scheme {
	case "file":
		filePath := u.Path
		if len(u.Host) == 2 && u.Host[1] == ':' {
			filePath = u.Host + filePath
		}
		return filepath.FromSlash(filePath), true
	case "":
		return sourceURL, true
	default:
		return "", false
	}
}
