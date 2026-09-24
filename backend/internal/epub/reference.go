package epub

import (
	"net/url"
	"path"
	"strings"
	"unicode/utf8"
)

// Reference is private publication data, not a client URL. Paths and fragments
// are decoded exactly once; only later normalization issues opaque public IDs.
// OCF names and decoded fragments must be valid UTF-8, so staging/JSON cannot
// silently replace invalid bytes and redirect a reference to a different target.
type Reference struct {
	Path     string
	Fragment string
}

func resolveReference(base, href string) (Reference, error) {
	u, err := url.Parse(href)
	if err != nil || u.IsAbs() || u.Host != "" || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery ||
		strings.HasPrefix(u.Path, "/") || strings.ContainsAny(u.Path, "\\\x00") || strings.ContainsRune(u.Fragment, '\x00') || !utf8.ValidString(u.Fragment) {
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
