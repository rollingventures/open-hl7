//go:build ignore

// genspec writes the reference OpenEMR ADT ConnectorSpec to
// configs/openemr-adt.connector.json so the example file stays in sync with the
// in-code reference. Run: go run tools/genspec/main.go
package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/rollingventures/open-hl7/internal/connectorgen"
)

func main() {
	spec := connectorgen.OpenEMRADTSpec("openemr-adt", "127.0.0.1:2576")
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll("configs", 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("configs/openemr-adt.connector.json", append(data, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Println("wrote configs/openemr-adt.connector.json")
}
