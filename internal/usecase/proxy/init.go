package proxy

import (
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type HttpProxyService struct {
	certManager *CertManager
}

func NewHttpProxyService() *HttpProxyService {
	cm, err := NewCertManager(defaultCACertPath, defaultCAKeyPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize certificate manager: %v", err)
	}

	return &HttpProxyService{
		certManager: cm,
	}
}

func (h *HttpProxyService) HandleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	r.RequestURI = ""
	r.Header.Del("Proxy-Connection")

	targetURL := r.URL.String()

	client := &http.Client{
		Timeout: 30 * time.Second,
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
