package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
)

func main() {
	action := flag.String("action", "integrity", "maintenance action: integrity or backup")
	database := flag.String("database", "data/frameflow.db", "path to the SQLite database")
	output := flag.String("output", "", "destination path for a backup")
	flag.Parse()
	if _, err := os.Stat(*database); err != nil {
		log.Fatalf("database: %v", err)
	}
	repository, err := sqlite.Open(*database)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.Close()
	switch *action {
	case "integrity":
		err = repository.VerifyIntegrity()
	case "backup":
		err = repository.Backup(*output)
	default:
		err = fmt.Errorf("unsupported action %q", *action)
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s completed for %s\n", *action, *database)
}
