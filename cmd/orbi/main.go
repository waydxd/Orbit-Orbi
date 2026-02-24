package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/agent"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/config"
	pb "github.com/waydxd/Orbit-Orbi/proto"
	"google.golang.org/grpc"
)

func main() {
	// Load .env if present so environment variables can be set from it.
	// This makes the app work when the user prefers dotenv files instead of
	// exporting variables in the shell.
	_ = godotenv.Load(".env")
	// Get configuration from environment variables
	calendarAddr := getEnv("CORE_CALENDAR_ADDR", getEnv("CALENDAR_SERVICE_ADDR", "localhost:50052"))
	// For very sensitive values prefer reading from a file (docker secrets).
	// Use OPENAI_API_KEY_FILE or REDIS_PASSWORD_FILE to point to secret files.
	openAIKey := getSecretEnv("OPENAI_API_KEY", "")
	model := getEnv("OPENAI_MODEL", "gpt-3.5-turbo")
	agentAddr := getEnv("AGENT_GRPC_ADDR", getEnv("AGENT_SERVICE_ADDR", "0.0.0.0:50042"))
	httpAddr := getEnv("AGENT_HTTP_ADDR", "0.0.0.0:8088")
	baseURL := getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1/")
	redisAddr := getEnv("REDIS_ADDR", "")
	redisPassword := getSecretEnv("REDIS_PASSWORD", "")
	redisDB := 0 // Default to DB 0

	if openAIKey == "" {
		log.Println("Warning: OPENAI_API_KEY not set. Agent will not function without it.")
		log.Println("If you're using Docker secrets, set OPENAI_API_KEY_FILE to the secret file path.")
		log.Println("Please set the environment variable before running in production.")
	}

	// Create agent configuration
	cfg := config.Config{
		CalendarServiceAddr: calendarAddr,
		OpenAIAPIKey:        openAIKey,
		Model:               model,
		BaseURL:             baseURL,
		RedisAddr:           redisAddr,
		RedisPassword:       redisPassword,
		RedisDB:             redisDB,
	}

	// Initialize the Orbi agent
	log.Println("Starting Orbi agent...")
	ag, err := agent.NewCalendarAgent(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	defer func() { _ = ag.Close() }()

	// Create gRPC server and register services
	grpcServer := grpc.NewServer()
	agentServer := orbi.NewAgentServer(ag)
	pb.RegisterAgentServiceServer(grpcServer, agentServer)

	// Start gRPC server in a goroutine
	go func() {
		listener, err := net.Listen("tcp", agentAddr)
		if err != nil {
			log.Fatalf("Failed to listen on %s: %v", agentAddr, err)
		}
		log.Printf("AgentService gRPC listening on %s", agentAddr)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	// Start HTTP health server
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ready, reason := agentServer.IsReady()
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(reason))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	go func() {
		log.Printf("HTTP health listening on %s", httpAddr)
		if err := http.ListenAndServe(httpAddr, mux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Mark agent as ready immediately - calendar client will be lazily initialized
	// when Core first calls ProcessMessage
	agentServer.SetReady(true, "ready; calendar client will connect on demand")
	log.Println("Agent is ready to accept connections")

	log.Println("Agent running in server mode. Clients can connect via gRPC to process messages.")

	// Keep the server running indefinitely
	select {}
}

// getSecretEnv retrieves a sensitive environment variable from a file
// if an env var with the suffix _FILE is set (useful for Docker secrets). It
// falls back to the regular environment variable or the provided default.
func getSecretEnv(key, defaultValue string) string {
	// 1) If an env var pointing to a secret file is present, prefer it.
	if filePath := os.Getenv(key + "_FILE"); filePath != "" {
		if b, err := os.ReadFile(filePath); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				log.Println("Using secret from file for", key)
				return s
			}
		} else {
			log.Printf("Warning: unable to read secret file %s for %s: %v", filePath, key, err)
		}
	}

	// 2) Fallback to the environment variable value (if any) or the provided default.
	return getEnv(key, defaultValue)
}

// getEnv retrieves an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
