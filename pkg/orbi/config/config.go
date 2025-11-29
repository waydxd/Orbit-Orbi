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
	Timezone            string // Timezone for datetime parsing, e.g., "Asia/Hong_Kong". Defaults to UTC if empty or invalid.
}
