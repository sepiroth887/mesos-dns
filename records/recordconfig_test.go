package records

import "testing"

func TestConfigValidates(t *testing.T) {
	validateStaticEntryFile("./sample-static.json")
}
