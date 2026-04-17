package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// insecureDefaults lists known placeholder values that must never reach production.
var insecureDefaults = map[string]string{
	"NODE_SECRET":        "default_node_secret",
	"PROXY_SECRET":       "default_buyer_secret",
	"ADMIN_SECRET":       "default_admin_secret",
	"GATEWAY_JWT_SECRET": "change-this-high-entropy-jwt-secret",
}

type Config struct {
	SupabaseURL         string
	Port                string
	ProxySecret         string
	NodeSecret          string
	AdminSecret         string
	RatePerGB           string
	// PoP heartbeat emission: Exra tokens minted per heartbeat tick.
	PopEmissionPerHeartbeat string
	RedisURL                string
	ExraMaxSupply        string
	ExraEpochSize        string // tokens per epoch (supply-based halving)
	ExraPolicyFinalized     bool
	GatewayJWTSecret        string
	PeaqRPC                 string
	ControlPort             string
	OracleID                string // Unique name of this oracle instance
	OracleNodes             int    // Total number of oracles in consensus
}

// LoadConfig reads the .env file and extracts configurations into the Config struct.
func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found or error reading it, using environment variables")
	}

	cfg := &Config{
		SupabaseURL:             getEnv("SUPABASE_URL", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"),
		Port:                    getEnv("PORT", "8080"),
		ProxySecret:             getEnv("PROXY_SECRET", "default_buyer_secret"),
		NodeSecret:              getEnv("NODE_SECRET", "default_node_secret"),
		AdminSecret:             getEnv("ADMIN_SECRET", "default_admin_secret"),
		RatePerGB:               getEnv("RATE_PER_GB", "0.30"),
		PopEmissionPerHeartbeat: getEnv("POP_EMISSION_PER_HEARTBEAT", "0.000050"),
		RedisURL:                getEnv("REDIS_URL", ""),
		ExraMaxSupply:       getEnv("EXRA_MAX_SUPPLY", "1000000000"),
		ExraEpochSize:       getEnv("EXRA_EPOCH_SIZE", "100000000"),
		ExraPolicyFinalized: getEnv("EXRA_POLICY_FINALIZED", "true") == "true",
		GatewayJWTSecret:    getEnv("GATEWAY_JWT_SECRET", "change-this-high-entropy-jwt-secret"),
		PeaqRPC:             getEnv("PEAQ_RPC", "https://rpc.krest.peaq.network"),
		ControlPort:         getEnv("CONTROL_PORT", "8081"),
		OracleID:            getEnv("ORACLE_ID", "oracle-master"),
		OracleNodes:         getEnvInt("ORACLE_NODES", 3),
	}

	// Warn loudly if critical secrets are still set to insecure placeholder values.
	for envKey, placeholder := range insecureDefaults {
		if getEnv(envKey, "") == placeholder {
			log.Printf("WARNING: %s is set to an insecure default value. Set a strong secret before going to production.", envKey)
		}
	}

	return cfg
}

func getEnvInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
