package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	"io"
	_ "io"
	"log"
	"net/http"
	url2 "net/url"
	"os"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	_ "go.opentelemetry.io/otel/trace"
)

type ViaCEPResponse struct {
	Localidade string `json:"localidade"`
	Erro       bool   `json:"erro"`
}

type WeatherAPIResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

type WeatherResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func celsiusToFahrenheit(c float64) float64 {
	return c*1.8 + 32
}

func celsiusToKelvin(c float64) float64 {
	return c + 273
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
			semconv.ServiceNameKey.String("weather-api-wrapper"),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

func handleWeather(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("weather-api-wrapper")
	ctx, span := tracer.Start(ctx, "fetch-real-weather")
	defer span.End()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CEP string `json:"cep"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// ViaCEP request with tracing
	_, viaCEPSpan := tracer.Start(ctx, "viacep-request")
	viaCEPResp, err := http.Get(fmt.Sprintf("https://viacep.com.br/ws/%s/json/", req.CEP))
	if err != nil {
		viaCEPSpan.RecordError(err)
		w.WriteHeader(http.StatusInternalServerError)
		viaCEPSpan.End()
		return
	}
	defer viaCEPResp.Body.Close()
	viaCEPSpan.End()

	var viaCEPData ViaCEPResponse
	if err := json.NewDecoder(viaCEPResp.Body).Decode(&viaCEPData); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if viaCEPData.Erro {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "can not find zipcode"})
		return
	}

	// WeatherAPI request with tracing
	_, weatherSpan := tracer.Start(ctx, "weather-request")
	weatherAPIKey := os.Getenv("WEATHER_API_KEY")
	weatherResp, err := http.Get(fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s",
		weatherAPIKey, url2.QueryEscape(viaCEPData.Localidade)))
	if err != nil {
		weatherSpan.RecordError(err)
		w.WriteHeader(http.StatusInternalServerError)
		weatherSpan.End()
		return
	}
	defer weatherResp.Body.Close()
	weatherSpan.End()

	var weatherData WeatherAPIResponse
	if err := json.NewDecoder(weatherResp.Body).Decode(&weatherData); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tempC := weatherData.Current.TempC
	response := WeatherResponse{
		City:  viaCEPData.Localidade,
		TempC: tempC,
		TempF: celsiusToFahrenheit(tempC),
		TempK: celsiusToKelvin(tempC),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func initEnv() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
}

func main() {
	initEnv()

	tp, err := initTracer()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	http.HandleFunc("/", handleWeather)
	fmt.Println("Weather API wrapper starting on port 8081...")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}
