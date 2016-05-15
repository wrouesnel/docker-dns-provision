/*
	This utility launches and caches docker container information from DNS TXT
	records .

	The concept here is that your immutable system on boot queries it's hostname
	for infrastructure container deployment, and then caches those results as
	configured unit files. Once launched, it doesn't change until a reboot
	at which point unit information is updated (though this is not intended
	as a full configuration suite).
 */

package main

import (
	"net"
	"os"
	"strings"

	//"github.com/samalba/dockerclient"

	"gopkg.in/alecthomas/kingpin.v2"
	"github.com/wrouesnel/go.log"
)

var (
	prefix string = kingpin.Flag("dns-prefix", "Prefixed name for DNS configuration records. Prepended to the hostname.").Default("containers.docker").String()
	unitFilePrefix = kingpin.Flag("unit-file-prefix", "Prefix for DNS provisioned unit files").Default("dns-provisioned").String()
	unitFilePath = kingpin.Flag("unit-file-dir", "Path to the systemd unit file directory").Default("/etc/systemd/system").ExistingDir()
	dockerSocket = kingpin.Flag("docker-socket", "Path to the docker socket").Default("/run/docker.sock")
	hostname = kingpin.Flag("hostname", "Hostname to query as. Defaults to system hostname.").String()
	inheritance = kingpin.Flag("inheritance", "When enabled, always runs a recursive query and merges container configs").Bool()
)

func main() {
	kingpin.Parse()

	// If no hostname, get the OS hostname. If that fails, then we can't really
	// do anything.
	if *hostname == "" {
		var err error
		*hostname, err = os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Infoln("Using hostname", *hostname)

	// Query the DNS containers
	log.Debugln("Starting DNS query")

	containers := txtRecords(*prefix, *hostname, *inheritance)

	log.Infoln("DNS specifies containers:", containers)
}

// Queries down the chain of possible hostnames and returns a map of docker
// containers we need to query for configuration.
func txtRecords(prefix string, hostname string, canInherit bool) []string {
	// Holds the deduplicated set of containers this host has config for
	containers := make(map[string]interface{})

	// Split the hostname up into fragments
	hostParts := strings.Split(hostname, ".")

	for idx, _ := range hostParts {
		// Calculate the fragment
		domain := strings.Join(hostParts[idx:], ".")
		// Determine the full DNS name with the config prefix
		dnsName := strings.Join(prefix, domain)

		txt, err := net.LookupTXT(dnsName)
		if err != nil {
			log.Debugln("Failed querying", dnsName, err)
		} else {
			log.Debugln("Lookup", dnsName, "found containers", txt)
			for _, containerName := range txt {
				containers[containerName] = nil
			}
			// If inheritance disabled, then stop querying once we get a result
			if !canInherit {
				break
			}
		}
	}

	return stringMapKeys(containers)
}
