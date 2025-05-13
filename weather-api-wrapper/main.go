package main

import (
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	_ "io"
	"log"
	"net/http"
	url2 "net/url"
	"os"
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

func handleWeather(w http.ResponseWriter, r *http.Request) {
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

	viaCEPResp, err := http.Get(fmt.Sprintf("https://viacep.com.br/ws/%s/json/", req.CEP))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer viaCEPResp.Body.Close()

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

	weatherAPIKey := os.Getenv("WEATHER_API_KEY")
	weatherResp, err := http.Get(fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s", weatherAPIKey, url2.QueryEscape(viaCEPData.Localidade)))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer weatherResp.Body.Close()

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
	http.HandleFunc("/", handleWeather)
	fmt.Println("Weather API wrapper starting on port 8081...")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}
