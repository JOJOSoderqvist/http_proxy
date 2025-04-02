package main

import (
	proxyServer "simple_proxy/internal/apps/proxy"
	proxyDelivery "simple_proxy/internal/delivery/proxy"
	proxyService "simple_proxy/internal/usecase/proxy"
)

func main() {
	httpProxyService := proxyService.NewHttpProxyService()
	httpProxyDelivery := proxyDelivery.NewHttpProxyDelivery(httpProxyService)

	server := proxyServer.NewHttpProxyServer(httpProxyDelivery)

	server.Run()
}
