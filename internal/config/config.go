package config

type Config struct {
	ServerAddr         string `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`
	BaseURL            string `env:"BASE_URL" envDefault:"http://localhost:8080"`
	FileStoragePath    string `env:"FILE_STORAGE_PATH"`
	SecretKey          string `env:"SHORTENER_SECRET_KEY" envDefault:"We learn Go language"`
	CookieAuthName     string `env:"COOKIE_ATUH_NAME" envDefault:"shortenerId"`
	DBConnectionString string `env:"DATABASE_DSN" envDefault:"postgres://postgres:mypassword@localhost:5432/postgres"`
}
