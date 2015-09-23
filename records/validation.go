package records

import (
	"fmt"
	"net"
	"os"
	"regexp"
)

// ValidFQDNRegex can be used to validate fqdn strings for A records
var ValidFQDNRegex = "^[a-zA-Z0-9][a-zA-Z0-9-\\.]{1,61}$"

// ValidSRVRegex can be used to validate host strings for SRV records
var ValidSRVRegex = "^[a-zA-Z0-9_][a-zA-Z0-9-._]{1,61}$"

// ValidHostPortRegex can validate Host:Port pairs (though it allow anychar as host for now and ports > 65365 < 99999)
var ValidHostPortRegex = "[a-zA-Z0-9\\.]+:[0-9]{1,5}"

func validateEnabledServices(c *Config) error {
	if !c.DNSOn && !c.HTTPOn {
		return fmt.Errorf("Either DNS or HTTP server should be on")
	}
	if len(c.Masters) == 0 && c.Zk == "" {
		return fmt.Errorf("specify mesos masters or zookeeper in config.json")
	}
	return nil
}

// validateMasters checks that each master in the list is a properly formatted host:ip pair.
// duplicate masters in the list are not allowed.
// returns nil if the masters list is empty, or else all masters in the list are valid.
func validateMasters(ms []string) error {
	if len(ms) == 0 {
		return nil
	}
	valid := make(map[string]struct{}, len(ms))
	for i, m := range ms {
		h, p, err := net.SplitHostPort(m)
		if err != nil {
			return fmt.Errorf("illegal host:port specified for master %q", ms[i])
		}
		// normalize ipv6 addresses
		if ip := net.ParseIP(h); ip != nil {
			h = ip.String()
			m = h + "_" + p
		}
		//TODO(jdef) distinguish between intended hostnames and invalid ip addresses
		if _, found := valid[m]; found {
			return fmt.Errorf("duplicate master specified: %v", ms[i])
		}
		valid[m] = struct{}{}
	}
	return nil
}

// validateResolvers checks that each resolver in the list is a properly formatted IP address.
// duplicate resolvers in the list are not allowed.
// returns nil if the resolver list is empty, or else all resolvers in the list are valid.
func validateResolvers(rs []string) error {
	if len(rs) == 0 {
		return nil
	}
	ips := make(map[string]struct{}, len(rs))
	for _, r := range rs {
		ip := net.ParseIP(r)
		if ip == nil {
			return fmt.Errorf("illegal IP specified for resolver %q", r)
		}
		ipstr := ip.String()
		if _, found := ips[ipstr]; found {
			return fmt.Errorf("duplicate resolver IP specified: %v", r)
		}
		ips[ipstr] = struct{}{}
	}
	return nil
}

func validateStaticEntryFile(sef string) (StaticEntryConfig, error) {
	if len(sef) == 0 {
		return StaticEntryConfig{}, nil
	}

	if _, err := os.Stat(sef); os.IsNotExist(err) {
		return StaticEntryConfig{}, fmt.Errorf("StaticEntryFile not found: %s", sef)
	}

	conf, err := ParseStaticConfig(sef)

	for _, entry := range conf.Entries {
		switch entry.Type {
		case "A":
			ip := net.ParseIP(entry.Value)
			if ip == nil {
				return conf, fmt.Errorf("Invalid IP on StaticEntry: %q", entry.Value)
			}
			if match, _ := regexp.MatchString(ValidFQDNRegex, entry.Fqdn); !match {
				return conf, fmt.Errorf("Invalid FQDN: %s", entry.Fqdn)
			}
			break
		case "SRV":
			if match, _ := regexp.MatchString(ValidSRVRegex, entry.Fqdn); !match {
				return conf, fmt.Errorf("Invalid SRV FQDN: %s", entry.Fqdn)
			}
			if match, _ := regexp.MatchString(ValidHostPortRegex, entry.Value); !match {
				return conf, fmt.Errorf("Invalid (Host:Port) tuple: %s", entry.Value)
			}
			break
		default:
			return conf, fmt.Errorf("Unsupported Record Type: %s", entry.Type)
		}
	}

	return conf, err
}

// validateIPSources checks validity of ip sources
func validateIPSources(srcs []string) error {
	if len(srcs) == 0 {
		return fmt.Errorf("empty ip sources")
	}
	if len(srcs) != len(unique(srcs)) {
		return fmt.Errorf("duplicate ip source specified")
	}
	for _, src := range srcs {
		switch src {
		case "host", "docker", "mesos", "netinfo":
		default:
			return fmt.Errorf("invalid ip source %q", src)
		}
	}

	return nil
}
