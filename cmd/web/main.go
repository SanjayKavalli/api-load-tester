package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"go-api-load-tester/internal/web"
)

func main() {
	port := flag.Int("port", 8080, "Local web interface port")
	ui := flag.String("ui", "both", "Enabled interfaces: both, classic, balanced")
	defaultUI := flag.String("default-ui", "", "Default interface: classic or balanced (defaults to an enabled interface)")
	flag.Parse()
	handler, err := web.NewHandlerWithUI(web.UIOptions{Mode: *ui, Default: *defaultUI})
	if err != nil {
		log.Fatal(err)
	}
	address := fmt.Sprintf("127.0.0.1:%d", *port)
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Load tester: http://%s", address)
	log.Fatal(server.ListenAndServe())
}
