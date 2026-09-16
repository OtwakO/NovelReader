package epub

import (
	"net/url"
	"path"
	"strings"
)

// Reference is private publication data, not a client URL. Paths and fragments
// are decoded exactly once; only later normalization issues opaque public IDs.
type Reference struct {
	Path     string
	Fragment string
}

func resolveReference(base, href string) (Reference, error) {
	u, err := url.Parse(href)
	if err != nil || u.IsAbs() || u.Host != "" || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery ||
		strings.HasPrefix(u.Path, "/") || strings.ContainsAny(u.Path, "\\\x00") || strings.ContainsRune(u.Fragment, '\x00') {
		return Reference{}, ErrReference
	}
	name := base
	if u.Path != "" {
		name = path.Join(path.Dir(base), u.Path)
	}
	if !validEntryName(name) {
		return Reference{}, ErrReference
	}
	return Reference{Path: name, Fragment: u.Fragment}, nil
}
