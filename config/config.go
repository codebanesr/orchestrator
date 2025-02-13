package config

type Config struct {
  TraefikNetwork  string
  Domain          string
  LetsEncryptEmail string
  AuthCredentials string // Basic auth in htpasswd format
}

func Load() *Config {
  return &Config{
    TraefikNetwork:  "traefik_network",
    Domain:          "yourdomain.com",
    LetsEncryptEmail: "admin@yourdomain.com",
    AuthCredentials: "user:$apr1$...", // Generated htpasswd
  }
}