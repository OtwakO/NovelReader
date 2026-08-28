package booksource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const MaxCollectionDocumentBytes = 50 * 1024 * 1024

type RemoteDocument struct {
	Body         []byte
	ETag         string
	LastModified string
	NotModified  bool
}

type RemoteLoader struct {
	client *http.Client
}

func NewRemoteLoader() *RemoteLoader {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, address := range addresses {
				if !publicCollectionIP(address.IP) {
					return nil, fmt.Errorf("booksource: collection URL resolves to a private address")
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
		},
		TLSHandshakeTimeout: 10 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("booksource: too many collection URL redirects")
		}
		return validateCollectionURL(request.URL)
	}
	return &RemoteLoader{client: client}
}

func (l *RemoteLoader) Load(ctx context.Context, rawURL, etag, lastModified string) (RemoteDocument, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return RemoteDocument{}, fmt.Errorf("booksource: parse collection URL: %w", err)
	}
	if err := validateCollectionURL(parsed); err != nil {
		return RemoteDocument{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return RemoteDocument{}, err
	}
	request.Header.Set("Accept", "application/json, text/plain;q=0.9")
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		request.Header.Set("If-Modified-Since", lastModified)
	}
	response, err := l.client.Do(request)
	if err != nil {
		return RemoteDocument{}, fmt.Errorf("booksource: fetch collection URL: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		return RemoteDocument{NotModified: true, ETag: etag, LastModified: lastModified}, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return RemoteDocument{}, fmt.Errorf("booksource: collection URL returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxCollectionDocumentBytes+1))
	if err != nil {
		return RemoteDocument{}, fmt.Errorf("booksource: read collection URL: %w", err)
	}
	if len(body) > MaxCollectionDocumentBytes {
		return RemoteDocument{}, fmt.Errorf("booksource: collection document exceeds 50 MiB")
	}
	return RemoteDocument{Body: body, ETag: response.Header.Get("ETag"), LastModified: response.Header.Get("Last-Modified")}, nil
}

func validateCollectionURL(value *url.URL) error {
	if value == nil || (value.Scheme != "http" && value.Scheme != "https") || value.Hostname() == "" || value.User != nil {
		return fmt.Errorf("booksource: collection URL must be a public HTTP or HTTPS URL without credentials")
	}
	return nil
}

func publicCollectionIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified() && !ip.IsMulticast()
}
