package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	_ "go.opentelemetry.io/otel/trace"
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
func initTracer() (*sdktrace.TracerProvider, error) {
	exporter, err := zipkin.New(
		"http://localhost:9411/api/v2/spans",
		zipkin.WithLogger(log.New(io.Discard, "", log.LstdFlags)),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("weather-man"),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

func handleZipCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("weather-man")
	ctx, span := tracer.Start(ctx, "fetch-weather-from-wrapper")
	defer span.End()

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

	// Create HTTP request with context
	body := bytes.NewBuffer([]byte(fmt.Sprintf(`{"cep":"%s"}`, req.CEP)))
	wrapperReq, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:8081", body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	wrapperReq.Header.Set("Content-Type", "application/json")

	// Forward to weather-api-wrapper
	wrapperResp, err := http.DefaultClient.Do(wrapperReq)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer wrapperResp.Body.Close()

	// Copy headers first
	for k, v := range wrapperResp.Header {
		w.Header()[k] = v
	}
	w.Header().Set("Content-Type", "application/json")

	// Then write status code
	w.WriteHeader(wrapperResp.StatusCode)

	// Copy response body
	if _, err := io.Copy(w, wrapperResp.Body); err != nil {
		log.Printf("Error copying response: %v", err)
	}
}

func main() {
	tp, err := initTracer()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	http.HandleFunc("/", handleZipCode)
	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
