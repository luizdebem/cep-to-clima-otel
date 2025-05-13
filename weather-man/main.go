package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
)

type ZipCodeRequest struct {
	CEP string `json:"cep"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func validateZipCode(cep string) bool {
	match, _ := regexp.MatchString(`^\d{8}$`, cep)
	return match
}

func handleZipCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req ZipCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !validateZipCode(req.CEP) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "invalid zipcode"})
		return
	}

	wrapperResp, err := http.Post(
		"http://localhost:8081",
		"application/json",
		bytes.NewBuffer([]byte(fmt.Sprintf(`{"cep":"%s"}`, req.CEP))),
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer wrapperResp.Body.Close()

	w.WriteHeader(wrapperResp.StatusCode)
	for k, v := range wrapperResp.Header {
		w.Header()[k] = v
	}

	if _, err := io.Copy(w, wrapperResp.Body); err != nil {
		log.Printf("Error copying response: %v", err)
	}
}

func main() {
	http.HandleFunc("/", handleZipCode)
	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
