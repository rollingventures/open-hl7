package connectorgen

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadSpec reads a ConnectorSpec from a JSON file. This is how an
// agent-generated (or hand-written) connector becomes a live channel.
func LoadSpec(path string) (ConnectorSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ConnectorSpec{}, fmt.Errorf("connectorgen: read spec: %w", err)
	}
	var spec ConnectorSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return ConnectorSpec{}, fmt.Errorf("connectorgen: parse spec: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return ConnectorSpec{}, err
	}
	return spec, nil
}

// Validate checks a spec is runnable.
func (s ConnectorSpec) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("connectorgen: spec missing name")
	}
	if len(s.MessageTypes) == 0 {
		return fmt.Errorf("connectorgen: spec %q has no message types", s.Name)
	}
	if s.Transport.Kind == TransportMLLP && s.Transport.Address == "" {
		return fmt.Errorf("connectorgen: spec %q mllp transport missing address", s.Name)
	}
	for _, fm := range s.FieldMaps {
		if _, _, _, err := parseHL7Path(fm.HL7Path); err != nil {
			return err
		}
	}
	return nil
}
