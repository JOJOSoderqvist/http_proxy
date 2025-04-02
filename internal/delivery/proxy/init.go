package proxy

import (
	"net/http"
)

type ProxyService interface {
	HandleHTTPRequest(w http.ResponseWriter, r *http.Request)
	HandleConnect(w http.ResponseWriter, r *http.Request)
}

type HttpProxyDelivery struct {
	proxyService ProxyService
}

func NewHttpProxyDelivery(service ProxyService) *HttpProxyDelivery {
	return &HttpProxyDelivery{
		proxyService: service,
	}
}

func (h *HttpProxyDelivery) HandleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		h.proxyService.HandleConnect(w, r)
		return
	}

	h.proxyService.HandleHTTPRequest(w, r)
}
