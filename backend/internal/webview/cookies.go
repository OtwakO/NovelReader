// Cookie conversion between Go SourceSession and the browser worker protocol.
package webview

import (
	"net/http"
	"time"
)

func toProtocolCookie(cookie *http.Cookie, rawURL string) protocolCookie {
	result := protocolCookie{
		Name: cookie.Name, URL: rawURL, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path,
		HTTPOnly: cookie.HttpOnly, Secure: cookie.Secure,
	}
	if !cookie.Expires.IsZero() {
		result.Expires = float64(cookie.Expires.Unix())
	}
	return result
}

func fromProtocolCookies(cookies []protocolCookie) []*http.Cookie {
	result := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		converted := &http.Cookie{
			Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path,
			HttpOnly: cookie.HTTPOnly, Secure: cookie.Secure,
		}
		if cookie.Expires > 0 {
			converted.Expires = time.Unix(int64(cookie.Expires), 0)
		}
		result = append(result, converted)
	}
	return result
}
