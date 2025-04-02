package proxy

import (
	"log"
	"net/http"
)

type HttpProxyDelivery interface {
	HandleProxy(w http.ResponseWriter, r *http.Request)
}

type HttpProxyServer struct {
	delivery HttpProxyDelivery
}

func NewHttpProxyServer(delivery HttpProxyDelivery) *HttpProxyServer {
	return &HttpProxyServer{
		delivery: delivery,
	}
}

func (h *HttpProxyServer) Run() {
	handler := http.HandlerFunc(h.delivery.HandleProxy)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	log.Println("Starting server on :8080")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("ListenAndServe error: %v", err)
	}
}
