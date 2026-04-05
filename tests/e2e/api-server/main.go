package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	port := "9091"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/customers/", customersHandler)
	mux.HandleFunc("/charges", chargesHandler)
	mux.HandleFunc("/search", searchHandler)
	mux.HandleFunc("/forms/submit", formsSubmitHandler)

	log.Printf("API test server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, logMiddleware(mux)))
}

// logMiddleware logs every incoming request to stdout.
func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.String(), r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

// customersHandler handles GET /customers/{id}.
func customersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /customers/{id}
	path := strings.TrimPrefix(r.URL.Path, "/customers/")
	id := strings.TrimSuffix(path, "/")
	if id == "" {
		http.Error(w, "missing customer id", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":    id,
		"email": "test@example.com",
		"name":  "Test Customer",
	})
}

// chargesHandler handles POST /charges.
func chargesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	amount, _ := body["amount"]
	currency, _ := body["currency"]

	writeJSON(w, http.StatusOK, map[string]any{
		"id":       "ch_test_123",
		"amount":   amount,
		"currency": currency,
		"status":   "succeeded",
	})
}

// searchHandler handles GET /search?q=<query>.
func searchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"query": query,
		"results": []map[string]string{
			{
				"id":    "1",
				"title": "Result for: " + query,
			},
		},
	})
}

// formsSubmitHandler handles POST /forms/submit with form-encoded body.
func formsSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form body", http.StatusBadRequest)
		return
	}

	received := make(map[string]string)
	for key, values := range r.PostForm {
		if len(values) > 0 {
			received[key] = values[0]
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"received": received,
		"status":   "ok",
	})
}

// writeJSON marshals v to JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
