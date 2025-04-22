package main

import (
	"log"
	proxyServer "simple_proxy/internal/apps/proxy"
	proxyDelivery "simple_proxy/internal/delivery/proxy"
	proxyService "simple_proxy/internal/usecase/proxy"
)

func main() {

	mongoURI := "mongodb://localhost:27017"
	mongoDB := "proxy_db"

	log.Printf("Connecting to MongoDB at %s, database: %s", mongoURI, mongoDB)

	httpProxyService := proxyService.NewHttpProxyService(mongoURI, mongoDB)
	httpProxyDelivery := proxyDelivery.NewHttpProxyDelivery(httpProxyService)

	server := proxyServer.NewHttpProxyServer(httpProxyDelivery)

	server.Run()
}
