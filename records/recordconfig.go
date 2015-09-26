package records

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

// StaticEntryConfig holds a slice of DNS entries
type StaticEntryConfig struct {

	// Entries contain all static DNS entries parsed via JSON
	Entries []StaticEntry
}

// StaticEntry represents a tuple of (FQDN, RecordType, VALUE)
type StaticEntry struct {
	Fqdn  string
	Type  string
	Value string
}

// ParseStaticConfig attempts to parse Entries from the passed jsonFile
func ParseStaticConfig(jsonFile string) (StaticEntryConfig, error) {

	conf := StaticEntryConfig{}

	// read configuration file
	path, err := filepath.Abs(jsonFile)
	if err != nil {
		return conf, err
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return conf, err
	}

	err = json.Unmarshal(b, &conf)
	if err != nil {
		return conf, err
	}

	return conf, nil
}
