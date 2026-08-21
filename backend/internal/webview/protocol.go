// JSON protocol shared by the Go WebView client and Patchright worker.
package webview

type protocolRequest struct {
	Version     int               `json:"version"`
	Probe       bool              `json:"probe,omitempty"`
	URL         string            `json:"url,omitempty"`
	Method      string            `json:"method"`
	Body        string            `json:"body,omitempty"`
	Charset     string            `json:"charset,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Cookies     []protocolCookie  `json:"cookies,omitempty"`
	WebJS       string            `json:"webJs,omitempty"`
	SourceRegex string            `json:"sourceRegex,omitempty"`
	DelayMS     int               `json:"delayMs,omitempty"`
	TimeoutMS   int               `json:"timeoutMs,omitempty"`
}

type protocolCookie struct {
	Name     string  `json:"name"`
	URL      string  `json:"url,omitempty"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain,omitempty"`
	Path     string  `json:"path,omitempty"`
	Expires  float64 `json:"expires,omitempty"`
	HTTPOnly bool    `json:"httpOnly,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
}

type protocolResponse struct {
	Version       int                 `json:"version"`
	StatusCode    int                 `json:"statusCode"`
	Headers       map[string][]string `json:"headers,omitempty"`
	Body          string              `json:"body,omitempty"`
	FinalURL      string              `json:"finalUrl,omitempty"`
	RedirectChain []string            `json:"redirectChain,omitempty"`
	Cookies       []protocolCookie    `json:"cookies,omitempty"`
	Error         string              `json:"error,omitempty"`
}
