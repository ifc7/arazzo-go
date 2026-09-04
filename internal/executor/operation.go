package executor

import (
	"fmt"
	"strings"

	libarazzo "github.com/pb33f/libopenapi/arazzo"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

type resolvedOperation struct {
	method  string
	path    string
	paramIn map[string]string
}

func parseOperationID(raw string) (sourceName, operationID string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	const prefix = "$sourceDescriptions."
	if strings.HasPrefix(raw, prefix) {
		rest := strings.TrimPrefix(raw, prefix)
		sourceName, operationID, ok := strings.Cut(rest, ".")
		if ok {
			return sourceName, operationID
		}
		return "", rest
	}
	if sourceName, operationID, ok := strings.Cut(raw, "."); ok && sourceName != "" && operationID != "" {
		return sourceName, operationID
	}
	return "", raw
}

func resolveOperation(sources []*libarazzo.ResolvedSource, preferred *libarazzo.ResolvedSource, operationID string) (*resolvedOperation, error) {
	sourceName, opID := parseOperationID(operationID)
	if opID == "" {
		return nil, fmt.Errorf("empty operationId")
	}

	if sourceName != "" {
		for _, src := range sources {
			if src != nil && src.Name == sourceName {
				if op, ok := findOperation(src.OpenAPIDocument, opID); ok {
					return op, nil
				}
			}
		}
	}
	if op, ok := findOperation(documentOf(preferred), opID); ok {
		return op, nil
	}
	for _, src := range sources {
		if op, ok := findOperation(documentOf(src), opID); ok {
			return op, nil
		}
	}
	if sourceName != "" {
		return nil, fmt.Errorf("could not find operationId %q in source %q", opID, sourceName)
	}
	return nil, fmt.Errorf("could not find operationId %q", opID)
}

func documentOf(src *libarazzo.ResolvedSource) *v3.Document {
	if src == nil {
		return nil
	}
	return src.OpenAPIDocument
}

func findOperation(doc *v3.Document, operationID string) (*resolvedOperation, bool) {
	if doc == nil || doc.Paths == nil || doc.Paths.PathItems == nil {
		return nil, false
	}
	for pathName, pathItem := range doc.Paths.PathItems.FromOldest() {
		if pathItem == nil {
			continue
		}
		for method, operation := range pathItem.GetOperations().FromOldest() {
			if operation == nil || operation.OperationId != operationID {
				continue
			}
			return &resolvedOperation{
				method:  strings.ToUpper(method),
				path:    pathName,
				paramIn: collectParamLocations(pathItem, operation),
			}, true
		}
	}
	return nil, false
}

func collectParamLocations(pathItem *v3.PathItem, operation *v3.Operation) map[string]string {
	locs := make(map[string]string)
	add := func(params []*v3.Parameter) {
		for _, p := range params {
			if p == nil || p.Name == "" || p.In == "" {
				continue
			}
			locs[p.Name] = strings.ToLower(p.In)
		}
	}
	if pathItem != nil {
		add(pathItem.Parameters)
	}
	if operation != nil {
		add(operation.Parameters)
	}
	return locs
}

func paramLocation(name string, pathTemplate string, known map[string]string) string {
	if strings.Contains(pathTemplate, "{"+name+"}") {
		return "path"
	}
	if loc, ok := known[name]; ok {
		return loc
	}
	return "query"
}
