package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	m, err := migrate.New("file://db/migrations", dbURL)
	if err != nil {
		log.Fatalf("failed to create migrator: %v", err)
	}
	defer m.Close()

	switch os.Args[1] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate up: %v", err)
		}
		fmt.Println("migrations applied")

	case "down":
		n := 1
		if len(os.Args) >= 3 {
			n, err = strconv.Atoi(os.Args[2])
			if err != nil || n < 1 {
				log.Fatalf("invalid step count: %q", os.Args[2])
			}
		}
		if err := m.Steps(-n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Printf("rolled back %d migration(s)\n", n)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("migrate version: %v", err)
		}
		fmt.Printf("version: %d  dirty: %v\n", version, dirty)

	case "seed":
		m.Close()
		runSeed(dbURL)

	default:
		printUsage()
		os.Exit(1)
	}
}

func runSeed(dbURL string) {
	sql, err := os.ReadFile("db/seeds/seed.sql")
	if err != nil {
		log.Fatalf("read seed file: %v", err)
	}

	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(context.Background(), string(sql)); err != nil {
		log.Fatalf("seed: %v", err)
	}

	fmt.Println("seed data inserted")
}

func printUsage() {
	fmt.Println("usage: go run ./cmd/migrate <command>")
	fmt.Println()
	fmt.Println("commands:")
	fmt.Println("  up           apply all pending migrations")
	fmt.Println("  down [n]     roll back n migrations (default 1)")
	fmt.Println("  version      show current migration version")
	fmt.Println("  seed         insert dummy data from db/seeds/seed.sql")
}
