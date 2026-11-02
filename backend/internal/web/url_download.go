package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const maxURLDownloadSize int64 = 10 << 30 // 10 GiB safety limit

// urlDownloadHandler downloads a public HTTP(S) resource on the server and
// stores it through ResourcePostHandler so normal path, permission and quota
// checks remain in force.
func urlDownloadHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	var input struct {
		URL    string `json:"url"`
		Path   string `json:"path"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid request")
	}
	if strings.TrimSpace(input.Path) == "" || strings.TrimSpace(input.Source) == "" {
		return http.StatusBadRequest, fmt.Errorf("target folder and source are required")
	}
	target, err := url.ParseRequestURI(strings.TrimSpace(input.URL))
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" || target.User != nil {
		return http.StatusBadRequest, fmt.Errorf("only valid HTTP and HTTPS URLs are supported")
	}
	client := publicURLHTTPClient()
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		return http.StatusBadRequest, err
	}
	request.Header.Set("User-Agent", "FileBrowser-Quantum-URLDownload")
	response, err := client.Do(request)
	if err != nil {
		return http.StatusBadGateway, fmt.Errorf("download failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return http.StatusBadGateway, fmt.Errorf("remote server returned %s", response.Status)
	}
	if response.ContentLength > maxURLDownloadSize {
		return http.StatusRequestEntityTooLarge, fmt.Errorf("download exceeds 10 GiB limit")
	}
	filename := path.Base(response.Request.URL.Path)
	if disposition := response.Header.Get("Content-Disposition"); disposition != "" {
		if _, params, parseErr := mime.ParseMediaType(disposition); parseErr == nil && params["filename"] != "" {
			filename = params["filename"]
		}
	}
	filename = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 || r < 0x20 || r == 0x7f {
			return '_'
		}
		return r
	}, filename))
	if filename == "" || filename == "." || filename == "/" {
		return http.StatusBadRequest, fmt.Errorf("could not determine a safe filename")
	}
	downloadPath := path.Join(input.Path, filename)
	query := url.Values{}
	query.Set("path", downloadPath)
	query.Set("source", input.Source)
	query.Set("override", "false")
	pipeReader, pipeWriter := io.Pipe()
	copyDone := make(chan error, 1)
	go func() {
		limited := io.LimitReader(response.Body, maxURLDownloadSize+1)
		n, copyErr := io.Copy(pipeWriter, limited)
		if copyErr == nil && n > maxURLDownloadSize {
			copyErr = fmt.Errorf("download exceeds 10 GiB limit")
		}
		_ = pipeWriter.CloseWithError(copyErr)
		copyDone <- copyErr
	}()
	postRequest := r.Clone(r.Context())
	postRequest.Method = http.MethodPost
	postRequest.URL = &url.URL{Path: "/api/resources", RawQuery: query.Encode()}
	postRequest.Body = pipeReader
	postRequest.ContentLength = response.ContentLength
	postRequest.Header = make(http.Header)
	postRequest.Header.Set("Content-Type", response.Header.Get("Content-Type"))
	status, postErr := ResourcePostHandler(w, postRequest, d)
	_ = pipeReader.Close()
	copyErr := <-copyDone
	if postErr == nil && copyErr != nil {
		return http.StatusBadGateway, fmt.Errorf("download stream failed: %w", copyErr)
	}
	return status, postErr
}

func publicURLHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Do not use environment-configured proxies: the proxy would resolve the
	// destination itself and bypass the IP checks in DialContext.
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 30 * time.Second
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, candidate := range ips {
			ip := candidate.IP
			if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
				return nil, fmt.Errorf("URL resolves to a non-public IP address")
			}
			conn, dialErr := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}
	return &http.Client{Transport: transport, Timeout: 0, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("unsupported redirect scheme")
		}
		return nil
	}}
}
