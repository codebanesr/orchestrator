package config

import "os"

type Config struct {
    TraefikNetwork   string
    Domain           string
    LetsEncryptEmail string
    AuthCredentials  string // Basic auth in htpasswd format
    DefaultVNCConfig VNCConfig
    BehindProxy bool
}

type VNCConfig struct {
    Password   string `json:"password"`
    Resolution string `json:"resolution"`
    ColDepth   int    `json:"colDepth"`
    ViewOnly   bool   `json:"viewOnly"`
    Display    string `json:"display"`
}

func Load() *Config {
    return &Config{
        TraefikNetwork:   "traefik_network",
        Domain:           "yourdomain.com",
        LetsEncryptEmail: "admin@yourdomain.com",
        AuthCredentials:  "user:$apr1$...", // Generated htpasswd
        BehindProxy:      os.Getenv("BEHIND_PROXY") == "true",
        DefaultVNCConfig: VNCConfig{
            Password:   "headless",
            Resolution: "1360x768",
            ColDepth:   24,
            ViewOnly:   false,
            Display:    ":1",
        },
    }
}