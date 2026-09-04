package executor

import (
	"strings"

	higharazzo "github.com/pb33f/libopenapi/datamodel/high/arazzo"
)

var wellKnownHeaders = map[string]struct{}{
	"authorization": {},
	"accept":        {},
	"content-type":  {},
	"cookie":        {},
	"x-csrf-token":  {},
	"x-request-id":  {},
}

// ArazzoParameterLocations maps parameter names to their Arazzo `in` values.
func ArazzoParameterLocations(doc *higharazzo.Arazzo) map[string]string {
	locs := make(map[string]string)
	if doc == nil {
		return locs
	}
	add := func(p *higharazzo.Parameter) {
		if p == nil || p.Name == "" || p.In == "" {
			return
		}
		locs[p.Name] = strings.ToLower(p.In)
	}
	if doc.Components != nil && doc.Components.Parameters != nil {
		for _, p := range doc.Components.Parameters.FromOldest() {
			add(p)
		}
	}
	for _, wf := range doc.Workflows {
		if wf == nil {
			continue
		}
		for _, p := range wf.Parameters {
			add(p)
		}
		for _, step := range wf.Steps {
			if step == nil {
				continue
			}
			for _, p := range step.Parameters {
				add(p)
			}
		}
	}
	return locs
}

func paramLocation(name, pathTemplate string, openapiIn, arazzoIn map[string]string) string {
	if strings.Contains(pathTemplate, "{"+name+"}") {
		return "path"
	}
	if loc, ok := lookupLocation(arazzoIn, name); ok {
		return loc
	}
	if loc, ok := lookupLocation(openapiIn, name); ok {
		return loc
	}
	if _, ok := wellKnownHeaders[strings.ToLower(name)]; ok {
		return "header"
	}
	return "query"
}

func lookupLocation(m map[string]string, name string) (string, bool) {
	if len(m) == 0 {
		return "", false
	}
	if loc, ok := m[name]; ok {
		return loc, true
	}
	for k, loc := range m {
		if strings.EqualFold(k, name) {
			return loc, true
		}
	}
	return "", false
}
