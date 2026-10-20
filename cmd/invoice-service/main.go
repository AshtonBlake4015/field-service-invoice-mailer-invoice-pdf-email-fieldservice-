package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"example.com/field-invoice-service/internal/fieldservice"
	api "example.com/field-invoice-service/internal/infrai"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	issuer := fieldservice.NewIssuer(api.New(key))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /invoices", func(w http.ResponseWriter, r *http.Request) {
		var order fieldservice.WorkOrder
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&order); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		delivery, err := issuer.Issue(r.Context(), order)
		if err != nil {
			status := http.StatusBadGateway
			var apiErr *api.Error
			switch {
			case errors.Is(err, fieldservice.ErrWorkOrderNotComplete):
				status = http.StatusConflict
			case errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500:
				status = apiErr.HTTPStatus
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, delivery)
	})

	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = ":8080"
	}
	log.Printf("invoice service listening on %s", address)
	log.Fatal(http.ListenAndServe(address, mux))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
