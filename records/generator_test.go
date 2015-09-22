package records

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records/labels"
)

func init() {
	logging.VerboseFlag = false
	logging.VeryVerboseFlag = false
	logging.SetupLogs()
}

func TestMasterRecord(t *testing.T) {
	// masterRecord(domain string, masters []string, leader string)
	type expectedRR struct {
		name  string
		host  string
		rtype string
	}
	tt := []struct {
		domain  string
		masters []string
		leader  string
		expect  []expectedRR
	}{
		{"foo.com", nil, "", nil},
		{"foo.com", nil, "@", nil},
		{"foo.com", nil, "1@", nil},
		{"foo.com", nil, "@2", nil},
		{"foo.com", nil, "3@4", nil},
		{"foo.com", nil, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master0.foo.com.", "6", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// single master: leader and fallback
		{"foo.com", []string{"6:7"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master0.foo.com.", "6", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// leader not in fallback list
		{"foo.com", []string{"8:9"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// duplicate fallback masters, leader not in fallback list
		{"foo.com", []string{"8:9", "8:9"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// leader that's also listed in the fallback list (at the end)
		{"foo.com", []string{"8:9", "6:7"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// duplicate leading masters in the fallback list
		{"foo.com", []string{"8:9", "6:7", "6:7"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// leader that's also listed in the fallback list (in the middle)
		{"foo.com", []string{"8:9", "6:7", "bob:0"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master.foo.com.", "bob", "A"},
				{"master0.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master2.foo.com.", "bob", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
	}
	for i, tc := range tt {
		rg := &RecordGenerator{}
		rg.As = make(rrs)
		rg.SRVs = make(rrs)
		t.Logf("test case %d", i+1)
		rg.masterRecord(tc.domain, tc.masters, tc.leader)
		if tc.expect == nil {
			if len(rg.As) > 0 {
				t.Fatalf("test case %d: unexpected As: %v", i+1, rg.As)
			}
			if len(rg.SRVs) > 0 {
				t.Fatalf("test case %d: unexpected SRVs: %v", i+1, rg.SRVs)
			}
		}
		expectedA := make(rrs)
		expectedSRV := make(rrs)
		for _, e := range tc.expect {
			found := rg.exists(e.name, e.host, e.rtype)
			if !found {
				t.Fatalf("test case %d: missing expected record: name=%q host=%q rtype=%s, As=%v", i+1, e.name, e.host, e.rtype, rg.As)
			}
			if e.rtype == "A" {
				expectedA[e.name] = append(expectedA[e.name], e.host)
			} else {
				expectedSRV[e.name] = append(expectedSRV[e.name], e.host)
			}
		}
		if !reflect.DeepEqual(rg.As, expectedA) {
			t.Fatalf("test case %d: expected As of %v instead of %v", i+1, expectedA, rg.As)
		}
		if !reflect.DeepEqual(rg.SRVs, expectedSRV) {
			t.Fatalf("test case %d: expected SRVs of %v instead of %v", i+1, expectedSRV, rg.SRVs)
		}
	}
}

func TestSanitizedSlaveAddress(t *testing.T) {
	spec := labels.ForRFC952()
	x := sanitizedSlaveAddress("1.2.3.4", spec)
	if x != "1.2.3.4" {
		t.Fatalf("unexpected slave address %q", x)
	}

	x = sanitizedSlaveAddress("localhost", spec)
	if x != "127.0.0.1" {
		t.Fatalf("unexpected slave address %q", x)
	}

	x = sanitizedSlaveAddress("unbelievable.domain.acme", spec)
	if x != "unbelievable.domain.acme" {
		t.Fatalf("unexpected slave address %q", x)
	}

	x = sanitizedSlaveAddress("unbelievable<>.domain!@#...acme", spec)
	if x != "unbelievable.domain.acme" {
		t.Fatalf("unexpected slave address %q", x)
	}
}

func TestYankPorts(t *testing.T) {
	p := "[31328-31328]"

	ports := yankPorts(p)

	if ports[0] != "31328" {
		t.Error("not parsing port")
	}
}

func TestMultipleYankPorts(t *testing.T) {
	p := "[31111-31111, 31113-31113]"

	ports := yankPorts(p)

	if len(ports) != 2 {
		t.Error("not parsing ports")
	}

	if ports[0] != "31111" {
		t.Error("not parsing port")
	}

	if ports[1] != "31113" {
		t.Error("not parsing port")
	}
}

func TestRangePorts(t *testing.T) {
	p := "[31115-31117]"

	ports := yankPorts(p)

	if len(ports) != 3 {
		t.Error("not parsing ports")
	}

	if ports[0] != "31115" {
		t.Error("not parsing port")
	}

	if ports[1] != "31116" {
		t.Error("not parsing port")
	}

	if ports[2] != "31117" {
		t.Error("not parsing port")
	}

}

func TestLeaderIP(t *testing.T) {
	l := "master@144.76.157.37:5050"

	ip := leaderIP(l)

	if ip != "144.76.157.37" {
		t.Error("not parsing ip")
	}
}

// ensure we are parsing what we think we are
func TestInsertState(t *testing.T) {

	var sj StateJSON

	b, err := ioutil.ReadFile("../factories/fake.json")
	if err != nil {
		t.Error("missing test data")
	}

	err = json.Unmarshal(b, &sj)
	if err != nil {
		t.Error(err)
	}
	sj.Leader = "master@144.76.157.37:5050"

	masters := []string{"144.76.157.37:5050"}
	spec := labels.ForRFC952()
	staticEntries := []StaticEntry{
		StaticEntry{Type: "A", Fqdn: "hello-static.mesos", Value: "127.0.0.2"},
		StaticEntry{Type: "SRV", Fqdn: "_hello-static._tcp.mesos", Value: "hello.static:443"},
	}
	rg := &RecordGenerator{}
	rg.InsertState(sj, "mesos", "mesos-dns.mesos.", "127.0.0.1", masters, staticEntries, spec)

	// ensure we are only collecting running tasks
	_, ok := rg.SRVs["_poseidon._tcp.marathon.mesos."]
	if ok {
		t.Error("should not find this not-running task - SRV record")
	}

	_, ok = rg.As["liquor-store.marathon.mesos."]
	if !ok {
		t.Error("should find this running task - A record")
	}

	_, ok = rg.As["poseidon.marathon.mesos."]
	if ok {
		t.Error("should not find this not-running task - A record")
	}

	_, ok = rg.As["master.mesos."]
	if !ok {
		t.Error("should find a running master - A record")
	}

	_, ok = rg.As["master0.mesos."]
	if !ok {
		t.Error("should find a running master0 - A record")
	}

	_, ok = rg.As["leader.mesos."]
	if !ok {
		t.Error("should find a leading master - A record")
	}

	_, ok = rg.SRVs["_leader._tcp.mesos."]
	if !ok {
		t.Error("should find a leading master - SRV record")
	}

	// test for 10 SRV names
	if len(rg.SRVs) != 11 {
		t.Error("not enough SRVs")
	}

	// test for 5 A names
	if len(rg.As) != 14 {
		t.Error("not enough As")
	}

	// ensure we translate the framework name as well
	_, ok = rg.As["some-box.chronoswithaspaceandmixe.mesos."]
	if !ok {
		t.Error("should find this task w/a space in the framework name - A record")
	}

	// ensure we find this SRV
	rrs := rg.SRVs["_liquor-store._tcp.marathon.mesos."]
	// ensure there are 3 RRDATA answers for this SRV name
	if len(rrs) != 3 {
		t.Error("not enough SRV records")
	}

	// ensure we don't find this as a SRV record
	rrs = rg.SRVs["_liquor-store.marathon.mesos."]
	if len(rrs) != 0 {
		t.Error("not a proper SRV record")
	}

	// ensure we find the static A record
	_, ok = rg.As["hello-static.mesos"]
	if !ok {
		t.Error("Missing static A record")
	}

	// ensure we find static SRV record
	rrs = rg.SRVs["_hello-static._tcp.mesos"]
	t.Log(rrs)
	if len(rrs) != 1 {
		t.Error("not enough static SRV records")
	}
}

// ensure we only generate one A record for each host
func TestNTasks(t *testing.T) {
	rg := &RecordGenerator{}
	rg.As = make(rrs)

	rg.insertRR("blah.mesos", "10.0.0.1", "A")
	rg.insertRR("blah.mesos", "10.0.0.1", "A")
	rg.insertRR("blah.mesos", "10.0.0.2", "A")

	k, _ := rg.As["blah.mesos"]

	if len(k) != 2 {
		t.Error("should only have 2 A records")
	}
}
