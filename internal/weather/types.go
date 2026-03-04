package weather

import "time"

type City struct {
	ID        string
	Name      string
	Latitude  float64
	Longitude float64
}

type Current struct {
	Temperature float64
	WeatherCode int
	Description string
	Icon        string
	WindSpeed   float64
	Humidity    float64
	Pressure    float64
	IsNight     bool
}

type Daily struct {
	Date        time.Time
	MinTemp     float64
	MaxTemp     float64
	WeatherCode int
	Description string
	Icon        string
}

type Hourly struct {
	Time        time.Time
	Temperature float64
	WeatherCode int
	Description string
	Icon        string
}

type WeatherData struct {
	CityID   string
	CityName string
	Current  Current
	Forecast []Daily
	Hourly   []Hourly
	Sunrise  time.Time
	Sunset   time.Time
}

