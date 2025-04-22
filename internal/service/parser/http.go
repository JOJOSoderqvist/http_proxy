package parser

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"simple_proxy/internal/model"
	"strings"
)

type HTTPParser struct{}

func NewHTTPParser() *HTTPParser {
	return &HTTPParser{}
}

func (p *HTTPParser) ParseRequest(r *http.Request) (*model.HTTPRequest, []byte, error) {
	req := &model.HTTPRequest{
		Method:      r.Method,
		Path:        r.URL.Path,
		QueryParams: make(map[string][]string),
		Headers:     make(map[string][]string),
		Cookies:     make(map[string]string),
		FormParams:  make(map[string][]string),
		TargetHost:  r.Host,
		ClientIP:    r.RemoteAddr,
		IsGzipped:   false,
	}

	for key, values := range r.URL.Query() {
		req.QueryParams[key] = values
	}

	for key, values := range r.Header {
		req.Headers[key] = values
	}

	for _, cookie := range r.Cookies() {
		req.Cookies[cookie.Name] = cookie.Value
	}

	if r.Header.Get("Content-Encoding") == "gzip" {
		req.IsGzipped = true
	}

	var bodyBytes []byte
	var err error

	if r.Body != nil {
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			return nil, nil, err
		}
		r.Body.Close()

		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		if req.IsGzipped {
			reader, err := gzip.NewReader(bytes.NewBuffer(bodyBytes))
			if err == nil {
				unzippedBytes, err := io.ReadAll(reader)
				reader.Close()
				if err == nil {
					req.Body = string(unzippedBytes)
				} else {
					req.Body = string(bodyBytes)
				}
			} else {
				req.Body = string(bodyBytes)
			}
		} else {
			req.Body = string(bodyBytes)
		}

		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			err = r.ParseForm()
			if err == nil {
				for key, values := range r.PostForm {
					req.FormParams[key] = values
				}
			}
		}
	}

	return req, bodyBytes, nil
}

func (p *HTTPParser) ParseResponse(resp *http.Response, requestID string) (*model.HTTPResponse, []byte, error) {
	requestIDObj, err := model.StringToObjectID(requestID)
	if err != nil {
		return nil, nil, err
	}

	res := &model.HTTPResponse{
		RequestID:     requestIDObj,
		StatusCode:    resp.StatusCode,
		Headers:       make(map[string][]string),
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: resp.ContentLength,
		IsGzipped:     false,
	}

	for key, values := range resp.Header {
		res.Headers[key] = values
	}

	if resp.Header.Get("Content-Encoding") == "gzip" {
		res.IsGzipped = true
	}

	var bodyBytes []byte
	if resp.Body != nil {
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, err
		}
		resp.Body.Close()

		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		if res.IsGzipped {
			reader, err := gzip.NewReader(bytes.NewBuffer(bodyBytes))
			if err == nil {
				unzippedBytes, err := io.ReadAll(reader)
				reader.Close()
				if err == nil {
					res.Body = string(unzippedBytes)
				} else {
					res.Body = string(bodyBytes)
				}
			} else {
				res.Body = string(bodyBytes)
			}
		} else {
			res.Body = string(bodyBytes)
		}
	}

	return res, bodyBytes, nil
}

func (p *HTTPParser) ModifyRequestForGzip(r *http.Request, bodyBytes []byte) {
	if r.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(bytes.NewBuffer(bodyBytes))
		if err == nil {
			unzippedBytes, err := io.ReadAll(reader)
			reader.Close()
			if err == nil {
				r.Body = io.NopCloser(bytes.NewBuffer(unzippedBytes))
				r.ContentLength = int64(len(unzippedBytes))
				r.Header.Del("Content-Encoding")
			}
		}
	}
}

func (p *HTTPParser) ModifyResponseForGzip(resp *http.Response, bodyBytes []byte) {
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(bytes.NewBuffer(bodyBytes))
		if err == nil {
			unzippedBytes, err := io.ReadAll(reader)
			reader.Close()
			if err == nil {
				resp.Body = io.NopCloser(bytes.NewBuffer(unzippedBytes))
				resp.ContentLength = int64(len(unzippedBytes))
				resp.Header.Del("Content-Encoding")
			}
		}
	}
}
