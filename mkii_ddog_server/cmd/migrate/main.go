package main

import (
	"database/sql"
	"log"

	"github.com/Nokodoko/mkii_ddog_server/cmd/api"
	"github.com/Nokodoko/mkii_ddog_server/cmd/config"
	"github.com/Nokodoko/mkii_ddog_server/cmd/db"
	_ "github.com/lib/pq"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// TODO:
//register users (or webhook agent) - 24:30 [if using] -- can they implement this in their backend and pass to the frontend
//register users defintely for rum unique users (uuids)
//create a type for monitors/hosts/serverless/integrations
//*automate /var/log to rotate log files - prevent filling disk space*

func main() {
	// Start Datadog APM tracer - automatically starts on every application restart
	tracer.Start(
		tracer.WithService(config.Envs.DDService),
		tracer.WithEnv(config.Envs.DDEnv),
		tracer.WithServiceVersion(config.Envs.DDVersion),
		tracer.WithAgentAddr(config.Envs.DDAgentHost+":8126"),
	)
	defer tracer.Stop()
	log.Printf("Datadog APM tracer started: service=%s env=%s version=%s agent=%s",
		config.Envs.DDService, config.Envs.DDEnv, config.Envs.DDVersion, config.Envs.DDAgentHost)

	db, err := db.SqlStorage(config.Envs)
	if err != nil {
		log.Fatal(err)
	}

	server := api.NewDdogServer(":8080", db)
	if err = server.Run(); err != nil {
		log.Fatal(err)
	}
}

func initStorage(db *sql.DB) {
	err := db.Ping()
	green := "\033[32m"
	if err != nil {
		log.Fatal(err)
	}
	log.Println("\x1B[3m" + green + "DB: Successfully Connected" + "\x1B[0m")
}
