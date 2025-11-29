package config

// Config holds the configuration for the Orbi agent
type Config struct {
	CalendarServiceAddr string
	OpenAIAPIKey        string
	Model               string
	BaseURL             string
	RedisAddr           string
	RedisPassword       string
	RedisDB             int
}
