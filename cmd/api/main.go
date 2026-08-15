package main

import (
	"goshop/internal/app"
	"goshop/internal/config"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func main() {
	loadEnv()

	cfg := config.NewConfig()

	application := app.NewApplication(cfg)

	if err := application.Run(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
}

func loadEnv() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	envPath := filepath.Join(cwd, ".env")

	err = godotenv.Load(envPath)
	if err != nil {
		log.Println("No .env found!")
	}
}
