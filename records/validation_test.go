package records

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestValidateMasters(t *testing.T) {
	for i, tc := range []validationTest{
		{nil, true},
		{[]string{}, true},
		{[]string{""}, false},
		{[]string{"", ""}, false},
		{[]string{"a"}, false},
		{[]string{"a:1234"}, true},
		{[]string{"a", "b"}, false},
		{[]string{"a:1", "b:1"}, true},
		{[]string{"1.2.3.4"}, false},
		{[]string{"1.2.3.4:5"}, true},
		{[]string{"1.2.3.4.5"}, false},
		{[]string{"1.2.3.4.5:6"}, true}, // no validation of hostnames
		{[]string{"1.2.3.4", "1.2.3.4"}, false},
		{[]string{"1.2.3.4:1", "1.2.3.4:1"}, false},
		{[]string{"1.2.3.4:1", "5.6.7.8:1"}, true},
		{[]string{"[2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b]:1"}, true},
		{[]string{"[2001:db8:3c4d:15::1a2f:1a2b]:1"}, true},
		{[]string{"[2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b]:1", "[2001:db8:3c4d:15::1a2f:1a2b]:1"}, false},
	} {
		validate(t, i+1, tc, validateMasters)
	}
}

func TestValidateResolvers(t *testing.T) {
	for i, tc := range []validationTest{
		{nil, true},
		{[]string{}, true},
		{[]string{""}, false},
		{[]string{"", ""}, false},
		{[]string{"a"}, false},
		{[]string{"a", "b"}, false},
		{[]string{"1.2.3.4"}, true},
		{[]string{"1.2.3.4.5"}, false},
		{[]string{"1.2.3.4", "1.2.3.4"}, false},
		{[]string{"1.2.3.4", "5.6.7.8"}, true},
		{[]string{"2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b"}, true},
		{[]string{"2001:db8:3c4d:15::1a2f:1a2b"}, true},
		{[]string{"2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b", "2001:db8:3c4d:15::1a2f:1a2b"}, false},
	} {
		validate(t, i+1, tc, validateResolvers)
	}
}

type validationTest struct {
	in    []string
	valid bool
}

func validate(t *testing.T, i int, tc validationTest, f func([]string) error) {
	switch err := f(tc.in); {
	case (err == nil && tc.valid) || (err != nil && !tc.valid):
		return // valid
	case tc.valid:
		t.Fatalf("test %d failed, unexpected error validating resolvers %v: %v", i, tc.in, err)
	default:
		t.Fatalf("test %d failed, expected validation error for resolvers(%d) %v", i, len(tc.in), tc.in)
	}
}

func TestValidateStaticEntryFile(t *testing.T) {
	// Testing valid first
	validJSON := "{ \"Entries\" : [{\"Fqdn\": \"hello.world\", \"Type\": \"A\", \"Value\": \"10.0.0.1\"}, {\"Fqdn\": \"_hello._tcp_.world\", \"Type\": \"SRV\", \"Value\": \"10.0.0.1:323\"}]}"

	CreateTempFile(validJSON, "/tmp/valid.json", t)
	conf, err := validateStaticEntryFile("/tmp/valid.json")

	if err != nil {
		t.Fatalf("Failed to parse valid json: %s", err)
	}

	if len(conf.Entries) != 2 {
		t.Fatalf("Incorrect entry set: expected %d got %d", 2, len(conf.Entries))
	}

	// Testing missing file
	conf, err = validateStaticEntryFile("/no/file")
	if err == nil {
		t.Fatalf("Expected Error")
	}

	// Testing empty StaticEntryFile fields
	conf, err = validateStaticEntryFile("")
	if err != nil {
		t.Fatalf("Expected no error but got: %s", err)
	}

	if len(conf.Entries) != 0 {
		t.Fatal("Expected empty entry set")
	}

	// Testing bad JSON
	invalidJSON := "{ \"BADFIELD\" : [{\"Fqdn\": \"hello.world\", \"Type\": \"A\", \"Value\": \"10.0.0.1Fqdn\": \"_hello._tcp_.world\", \"Type\": \"SRV\", \"Value\": \"10.0.0.1:323\"}]}"

	CreateTempFile(invalidJSON, "/tmp/invalid.json", t)
	conf, err = validateStaticEntryFile("/tmp/invalid.json")

	if err == nil {
		t.Fatal("Expected error")
	}

	// Testing bad Record data
	invalidFQDN := "{ \"Entries\" : [{\"Fqdn\": \"hello.world+-23\", \"Type\": \"A\", \"Value\": \"10.0.0.1\"}]}"
	invalidIP := "{ \"Entries\" : [{\"Fqdn\": \"hello.world\", \"Type\": \"A\", \"Value\": \"10.0.0.av1\"}]}"
	invalidSRV := "{ \"Entries\" : [{\"Fqdn\": \"_hello+110.!world\", \"Type\": \"SRV\", \"Value\": \"10.0.0.0:1234\"}]}"
	invalidHostPort := "{ \"Entries\" : [{\"Fqdn\": \"_hello._world\", \"Type\": \"SRV\", \"Value\": \"10.0.ase20.0:a1234\"}]}"

	CreateTempFile(invalidFQDN, "/tmp/invalid.json", t)
	conf, err = validateStaticEntryFile("/tmp/invalid.json")

	if err == nil {
		t.Fatal("Expected error")
	}

	CreateTempFile(invalidIP, "/tmp/invalid.json", t)
	conf, err = validateStaticEntryFile("/tmp/invalid.json")

	if err == nil {
		t.Fatal("Expected error")
	}

	CreateTempFile(invalidSRV, "/tmp/invalid.json", t)
	conf, err = validateStaticEntryFile("/tmp/invalid.json")

	if err == nil {
		t.Fatal("Expected error")
	}

	CreateTempFile(invalidHostPort, "/tmp/invalid.json", t)
	conf, err = validateStaticEntryFile("/tmp/invalid.json")

	if err == nil {
		t.Fatal("Expected error")
	}

	Cleanup("/tmp/invalid.json", "/tmp/valid.json")
}

func CreateTempFile(data, filename string, t *testing.T) {
	err := ioutil.WriteFile(filename, []byte(data), 0644)
	if err != nil {
		t.Fatalf("Failed to generate temp file: %s", err)
	}
}

func Cleanup(files ...string) {
	for _, file := range files {
		os.Remove(file)
	}
}
