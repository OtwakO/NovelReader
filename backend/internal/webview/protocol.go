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

type InteractiveViewport struct {
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	DeviceScaleFactor float64 `json:"deviceScaleFactor,omitempty"`
}

type interactiveRequest struct {
	URL        string              `json:"url"`
	Headers    map[string]string   `json:"headers,omitempty"`
	Cookies    []protocolCookie    `json:"cookies,omitempty"`
	Viewport   InteractiveViewport `json:"viewport,omitempty"`
	TimeoutMS  int                 `json:"timeoutMs,omitempty"`
	Save       bool                `json:"save,omitempty"`
	ReturnHTML bool                `json:"returnHtml,omitempty"`
}

type InteractiveInput struct {
	Type string  `json:"type"`
	X    float64 `json:"x,omitempty"`
	Y    float64 `json:"y,omitempty"`
	Text string  `json:"text,omitempty"`
	Key  string  `json:"key,omitempty"`
}

type InteractiveCloseResult struct {
	HTML string
}

type InteractiveFrame struct {
	SessionID string `json:"sessionId"`
	Image     string `json:"image,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
}

type interactiveResult struct {
	InteractiveFrame
	Version  int              `json:"version"`
	Closed   bool             `json:"closed,omitempty"`
	Cookies  []protocolCookie `json:"cookies,omitempty"`
	FinalURL string           `json:"finalUrl,omitempty"`
	HTML     string           `json:"html,omitempty"`
	Error    string           `json:"error,omitempty"`
}

type protocolResponse struct {
	UserAgent     string              `json:"userAgent,omitempty"` // browser default, returned only by probes
	Version       int                 `json:"version"`
	StatusCode    int                 `json:"statusCode"`
	Headers       map[string][]string `json:"headers,omitempty"`
	Body          string              `json:"body,omitempty"`
	FinalURL      string              `json:"finalUrl,omitempty"`
	RedirectChain []string            `json:"redirectChain,omitempty"`
	Cookies       []protocolCookie    `json:"cookies,omitempty"`
	Error         string              `json:"error,omitempty"`
}
