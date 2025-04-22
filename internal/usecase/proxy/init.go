package proxy

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"simple_proxy/internal/model"
	"simple_proxy/internal/repository/mongo"
	"simple_proxy/internal/service/parser"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HttpProxyService struct {
	certManager *CertManager
	parser      *parser.HTTPParser
	repository  *mongo.HTTPRepository
	params      []string // List of parameters to test
}

func loadParams(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var params []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		param := strings.TrimSpace(scanner.Text())
		if param != "" {
			params = append(params, param)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return params, nil
}

func generateRandomValue(length int) string {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "randomvalue123456789"
	}
	return hex.EncodeToString(bytes)
}

func isParameterReflected(paramName string, responseBody string) bool {
	return strings.Contains(responseBody, paramName)
}

func NewHttpProxyService(mongoURI, mongoDB string) *HttpProxyService {
	cm, err := NewCertManager(defaultCACertPath, defaultCAKeyPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize certificate manager: %v", err)
	}

	httpParser := parser.NewHTTPParser()

	repo, err := mongo.NewHTTPRepository(mongoURI, mongoDB)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize MongoDB repository: %v", err)
	}

	params, err := loadParams("params.txt")
	if err != nil {
		log.Printf("WARNING: Failed to load parameters from params.txt: %v", err)
		params = []string{}
	}

	log.Printf("Loaded %d parameters from params.txt", len(params))

	return &HttpProxyService{
		certManager: cm,
		parser:      httpParser,
		repository:  repo,
		params:      params,
	}
}

func (h *HttpProxyService) HandleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	parsedRequest, bodyBytes, err := h.parser.ParseRequest(r)
	if err != nil {
		log.Printf("Error parsing request: %v\n", err)
		http.Error(w, "Error parsing request", http.StatusInternalServerError)
		return
	}

	err = h.repository.SaveRequest(ctx, parsedRequest)
	if err != nil {
		log.Printf("Error saving request to MongoDB: %v\n", err)
	}

	r.RequestURI = ""
	r.Header.Del("Proxy-Connection")

	h.parser.ModifyRequestForGzip(r, bodyBytes)

	targetURL := r.URL.String()
	log.Printf("Forwarding request to %s\n", targetURL)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	if len(h.params) > 0 {
		originalURL := r.URL.String()
		baseURL := r.URL.Scheme + "://" + r.URL.Host + r.URL.Path

		for _, param := range h.params {
			randomValue := generateRandomValue(16)

			paramURL := baseURL
			if strings.Contains(baseURL, "?") {
				paramURL += "&" + param + "=" + randomValue
			} else {
				paramURL += "?" + param + "=" + randomValue
			}

			log.Printf("Testing parameter %s with URL: %s\n", param, paramURL)

			paramReq, err := http.NewRequest(r.Method, paramURL, nil)
			if err != nil {
				log.Printf("Error creating param-miner request for %s: %v\n", paramURL, err)
				continue
			}

			for key, values := range r.Header {
				for _, value := range values {
					paramReq.Header.Add(key, value)
				}
			}
			paramReq.Host = r.Host

			paramResp, err := client.Do(paramReq)
			if err != nil {
				log.Printf("Error performing param-miner request to %s: %v\n", paramURL, err)
				continue
			}

			paramRespBody, err := io.ReadAll(paramResp.Body)
			paramResp.Body.Close()
			if err != nil {
				log.Printf("Error reading param-miner response body: %v\n", err)
				continue
			}

			responseBodyStr := string(paramRespBody)
			if isParameterReflected(param, responseBodyStr) {
				log.Printf("FOUND REFLECTED PARAMETER: %s\n", param)
				log.Printf("Hidden parameter value: %s\n", randomValue)
			}
		}

		parsedURL, _ := url.Parse(originalURL)
		r.URL = parsedURL
	}

	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		log.Printf("Error creating new request for %s: %v\n", targetURL, err)
		http.Error(w, "Error creating request", http.StatusInternalServerError)
		return
	}

	req.Header = r.Header
	req.Host = r.Host

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error performing request to %s: %v\n", targetURL, err)
		http.Error(w, "Error forwarding request", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	log.Printf("Received response from %s: %d\n", targetURL, resp.StatusCode)

	parsedResponse, respBodyBytes, err := h.parser.ParseResponse(resp, parsedRequest.ID.Hex())
	if err != nil {
		log.Printf("Error parsing response: %v\n", err)
	} else {
		err = h.repository.SaveResponse(ctx, parsedResponse)
		if err != nil {
			log.Printf("Error saving response to MongoDB: %v\n", err)
		}
	}

	h.parser.ModifyResponseForGzip(resp, respBodyBytes)

	for key, values := range resp.Header {
		w.Header()[key] = values
	}

	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response body to client: %v\n", err)
	}
}

func (h *HttpProxyService) HandleConnect(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	reqID := primitive.NewObjectID()
	connectRequest := &model.HTTPRequest{
		ID:          reqID,
		Method:      r.Method,
		Path:        r.URL.Path,
		TargetHost:  r.Host,
		ClientIP:    r.RemoteAddr,
		Headers:     make(map[string][]string),
		Cookies:     make(map[string]string),
		QueryParams: make(map[string][]string),
	}

	for key, values := range r.Header {
		connectRequest.Headers[key] = values
	}

	for _, cookie := range r.Cookies() {
		connectRequest.Cookies[cookie.Name] = cookie.Value
	}

	err := h.repository.SaveRequest(ctx, connectRequest)
	if err != nil {
		log.Printf("Error saving CONNECT request to MongoDB: %v\n", err)
	}

	log.Printf("Handling CONNECT request for %s from %s\n", r.Host, r.RemoteAddr)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	if err != nil {
		log.Printf("Failed to send 200 OK to client %s: %v\n", clientConn.RemoteAddr(), err)
		return
	}
	log.Printf("Sent 200 OK for %s. Proceeding with MITM TLS handshake...\n", r.Host)

	connectResponse := &model.HTTPResponse{
		RequestID:     reqID,
		StatusCode:    200,
		Headers:       make(map[string][]string),
		Body:          "Connection established",
		ContentType:   "text/plain",
		ContentLength: 22,
	}

	err = h.repository.SaveResponse(ctx, connectResponse)
	if err != nil {
		log.Printf("Error saving CONNECT response to MongoDB: %v\n", err)
	}

	tlsConfig := &tls.Config{
		GetCertificate: h.certManager.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	}

	tlsClientConn := tls.Server(clientConn, tlsConfig)
	err = tlsClientConn.Handshake()
	if err != nil {
		log.Printf("TLS handshake with client %s (for %s) failed: %v\n", clientConn.RemoteAddr(), r.Host, err)
		return
	}
	log.Printf("TLS handshake with client %s successful.\n", clientConn.RemoteAddr())
	defer tlsClientConn.Close()

	targetHost := r.Host
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

	destTLSConfig := &tls.Config{
		ServerName: tlsClientConn.ConnectionState().ServerName,
	}
	if destTLSConfig.ServerName == "" {
		hostOnly, _, _ := net.SplitHostPort(targetHost)
		destTLSConfig.ServerName = hostOnly
	}

	destTLSConn, err := tls.DialWithDialer(dialer, "tcp", targetHost, destTLSConfig)
	if err != nil {
		log.Printf("Failed to establish TLS connection to destination %s: %v\n", targetHost, err)
		return
	}
	log.Printf("TLS connection to destination %s established.\n", targetHost)
	defer destTLSConn.Close()

	errChan := make(chan error, 2)
	go func() {
		_, copyErr := io.Copy(destTLSConn, tlsClientConn)
		if copyErr != nil && !errors.Is(copyErr, io.EOF) && !strings.Contains(copyErr.Error(), "use of closed network connection") {
			log.Printf("Error copying client->dest for %s: %v", targetHost, copyErr)
		} else {
			copyErr = nil
		}
		errChan <- copyErr
	}()
	go func() {
		_, copyErr := io.Copy(tlsClientConn, destTLSConn)
		if copyErr != nil && !errors.Is(copyErr, io.EOF) && !strings.Contains(copyErr.Error(), "use of closed network connection") {
			log.Printf("Error copying dest->client for %s: %v", targetHost, copyErr)
		} else {
			copyErr = nil
		}
		errChan <- copyErr
	}()

	<-errChan
	<-errChan
}
