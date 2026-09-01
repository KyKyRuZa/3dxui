package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv                  string
	Port                    string
	DatabaseURL             string
	RedisURL                string
	JWTSecret               string
	JWTPrivateKey           string
	CORSDomain              string
	BotToken                string
	BotUsername             string
	AdminAPISecret          string
	BotAPISecret            string
	PanelURL                string
	PanelUsername           string
	PanelPassword           string
	PanelAPIToken           string
	DefaultInboundIDs       []int
	DefaultGroup            string
	PanelPublicURL          string
	DefaultSubscriptionDays int
	ExpiryNotifyDays        int
	ReferralRewardDays      int
	ReferralSignupBonusDays int

	YookassaShopID    string
	YookassaSecretKey string
	YookassaAPIURL    string
	YookassaReturnURL string
	YookassaTestMode  bool
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/app/config")
	viper.AddConfigPath("./config")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		_ = err
	}

	cfg := &Config{
		AppEnv:                  viper.GetString("app_env"),
		Port:                    viper.GetString("port"),
		DatabaseURL:             viper.GetString("database_url"),
		RedisURL:                viper.GetString("redis_url"),
		JWTSecret:               viper.GetString("jwt_secret"),
		CORSDomain:              viper.GetString("cors_domain"),
		BotToken:                viper.GetString("bot_token"),
		BotUsername:             viper.GetString("bot_username"),
		AdminAPISecret:          viper.GetString("admin_api_secret"),
		BotAPISecret:            viper.GetString("bot_api_secret"),
		PanelURL:                viper.GetString("panel_url"),
		PanelUsername:           viper.GetString("panel_username"),
		PanelPassword:           viper.GetString("panel_password"),
		PanelAPIToken:           viper.GetString("panel_api_token"),
		DefaultGroup:            viper.GetString("default_group"),
		PanelPublicURL:          viper.GetString("panel_public_url"),
		DefaultSubscriptionDays: viper.GetInt("default_subscription_days"),
		ExpiryNotifyDays:        viper.GetInt("expiry_notify_days"),
		ReferralRewardDays:      viper.GetInt("referral_reward_days"),
		ReferralSignupBonusDays: viper.GetInt("referral_signup_bonus_days"),
		YookassaShopID:          viper.GetString("yookassa_shop_id"),
		YookassaSecretKey:       viper.GetString("yookassa_secret_key"),
		YookassaAPIURL:          viper.GetString("yookassa_api_url"),
		YookassaReturnURL:       viper.GetString("yookassa_return_url"),
		YookassaTestMode:        viper.GetBool("yookassa_test_mode") || strings.HasPrefix(viper.GetString("yookassa_secret_key"), "test_"),
	}

	if cfg.BotUsername == "" {
		cfg.BotUsername = "AutoColorsBot"
	}
	if cfg.DefaultGroup == "" {
		cfg.DefaultGroup = "Free"
	}
	if cfg.DefaultSubscriptionDays == 0 {
		cfg.DefaultSubscriptionDays = 2
	}
	if cfg.ExpiryNotifyDays == 0 {
		cfg.ExpiryNotifyDays = cfg.DefaultSubscriptionDays
	}
	if cfg.ReferralRewardDays == 0 {
		cfg.ReferralRewardDays = 7
	}
	if cfg.ReferralSignupBonusDays == 0 {
		cfg.ReferralSignupBonusDays = 2
	}

	if cfg.YookassaAPIURL == "" {
		cfg.YookassaAPIURL = "https://api.yookassa.ru/v3"
	}
	if cfg.YookassaReturnURL == "" {
		cfg.YookassaReturnURL = viper.GetString("public_origin")
	}
	if cfg.YookassaReturnURL == "" {
		cfg.YookassaReturnURL = viper.GetString("web_app_url")
	}

	inboundIDsStr := viper.GetString("default_inbound_ids")
	if inboundIDsStr != "" {
		parts := strings.Split(inboundIDsStr, ",")
		for _, p := range parts {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				cfg.DefaultInboundIDs = append(cfg.DefaultInboundIDs, n)
			}
		}
	}
	if len(cfg.DefaultInboundIDs) == 0 {
		cfg.DefaultInboundIDs = []int{1}
	}

	if cfg.Port == "" {
		cfg.Port = os.Getenv("PORT")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.AppEnv == "" {
		cfg.AppEnv = os.Getenv("APP_ENV")
	}
	if cfg.AppEnv == "" {
		cfg.AppEnv = "development"
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.RedisURL == "" {
		missing = append(missing, "REDIS_URL")
	}
	if cfg.BotToken == "" {
		missing = append(missing, "BOT_TOKEN")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

// IsProd reports whether the service runs in production mode.
func (c *Config) IsProd() bool { return c.AppEnv == "production" }

// CorsOrigins returns the list of allowed CORS origins.
func (c *Config) CorsOrigins() []string {
	if c.CORSDomain == "" {
		return nil
	}
	parts := strings.Split(c.CORSDomain, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

var ErrMissingConfig = errors.New("missing required configuration")
