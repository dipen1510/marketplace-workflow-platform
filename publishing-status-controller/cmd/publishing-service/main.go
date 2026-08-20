package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/publishing"
)

func main() {
	address := ":8080"

	service := publishing.NewService()

	server := &http.Server{
		Addr:    address,
		Handler: service.Handler(),
	}

	fmt.Printf(
		"Marketplace Publishing REST Service listening on %s\n",
		address,
	)

	if err := server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {

		log.Fatalf(
			"REST server failed: %v",
			err,
		)
	}
}
