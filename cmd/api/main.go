package main

import (
	"goshop/internal/app"
	"goshop/internal/config"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		panic(err)
	}

	// Initialize configuration
	cfg := config.NewConfig()

	// Initialize application
	application := app.NewApplication(cfg)

	// Start Server
	if err := application.Run(); err != nil {
		panic(err)
	}
}
