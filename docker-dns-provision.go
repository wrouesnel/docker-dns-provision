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

	//"github.com/fsouza/go-dockerclient"

	"gopkg.in/alecthomas/kingpin.v2"
	"github.com/wrouesnel/go.log"
	"os/exec"
	"flag"
	"github.com/kballard/go-shellquote"
	"encoding/base64"
)

var (
	prefix = kingpin.Flag("dns-prefix", "Prefixed name for DNS configuration records. Prepended to the hostname.").Default("containers.docker").String()
	//unitFilePrefix = kingpin.Flag("unit-file-prefix", "Prefix for DNS provisioned unit files").Default("dns-provisioned").String()
	//unitFilePath = kingpin.Flag("unit-file-dir", "Path to the systemd unit file directory").Default("/etc/systemd/system").ExistingDir()
	//dockerSocket = kingpin.Flag("docker-socket", "Path to the docker socket").Default("unix:///run/docker.sock").String()
	hostname = kingpin.Flag("hostname", "Hostname to query as. Defaults to system hostname.").String()
	dockerCmd = kingpin.Flag("docker-cmd", "Path to the docker command.").Default("docker").String()
	inheritance = kingpin.Flag("inheritance", "When enabled, always runs a recursive query and merges container configs").Bool()
	loglevel = kingpin.Flag("log-level", "Logging level").Default("info").String()
	// TODO: dnssec support - this is how we make this safe and secure
)

func main() {
	kingpin.Parse()
	flag.Set("log.level", *loglevel)
	//flag.Set("log.format", logformat)

	if _, err := exec.LookPath(*dockerCmd); err != nil {
		log.Fatalln("Supplied docker command is not executable in the current environment.")
	}

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

	containers := containerRecords(*prefix, *hostname, *inheritance)

	log.Infoln("DNS specifies containers:", containers)

	// First check for any containers which definitely should be removed (no
	// longer specified in DNS)
	namesOut, err := exec.Command(*dockerCmd, "ps", "-a", "--format", "{{ .Names }}" ,"-f",
		"label=docker-dns-provision.command").Output()
	if err != nil {
		log.Errorln("Failed to list containers - will not attempt cleanup:", err)
	} else {
		presentContainers := strings.Split(string(namesOut), "\n")
		for _, runningContainer := range presentContainers {
			log.Debugln("Containers we have labelled found:", presentContainers)
			if _, found := containers[runningContainer]; !found {
				log.Infoln("Container on host managed by docker-dns-provision",
					runningContainer, "not in provision data. Removing.")
				exec.Command(*dockerCmd, "kill", runningContainer).Run()
				exec.Command(*dockerCmd, "rm", runningContainer).Run()
			}
		}
	}

	for containerName, _ := range containers {
		log := log.With("containerName", containerName)

		commandLine := containerCommands(containerName, *prefix, *hostname)
		// Compute the b64 of this container's new command line
		theirb64cmd := base64.StdEncoding.EncodeToString([]byte(commandLine))

		if commandLine == "" {
			// Check if container with name exists already
			err := exec.Command(*dockerCmd, "inspect", "-f",
				`{{index .Config.Labels "docker-dns-provision.command" }}`,
				containerName ).Run()
			if err != nil {
				log.Debugln("Container did not exist, no need to try and remove.")
			} else {
				log.Infoln("Removing disabled container:", containerName)
				exec.Command(*dockerCmd, "kill", containerName).Run()
				exec.Command(*dockerCmd, "rm", containerName).Run()
			}
		} else {
			splitCmdLine, err := shellquote.Split(commandLine)
			if err != nil {
				log.Errorln("Could not split command line for container - skipping")
				continue
			}
			launchCommands := append(
				[]string{"run", "-d", "--name", containerName, "--label", "docker-dns-provision.command=" + theirb64cmd},
				splitCmdLine...)
			// Check if container with name exists already
			ourb64cmd_bytes, err := exec.Command(*dockerCmd, "inspect", "-f",
				`{{index .Config.Labels "docker-dns-provision.command" }}`,
				containerName ).Output()
			ourb64cmd := string(ourb64cmd_bytes) // Trim trailing white space
			ourb64cmd = strings.Trim(ourb64cmd, "\n")
			if err != nil {
				// Start unconditionally
				log.Infoln("Starting container")
				err := exec.Command(*dockerCmd, launchCommands...).Run();
				if err != nil {
					log.Errorln("Error starting container:", err)
				}
			} else {
				log.Debugln("Comparing command encodings:", ourb64cmd, theirb64cmd)
				if ourb64cmd != theirb64cmd {
					log.Infoln("Container configuration has changed - removing old container.")
					exec.Command(*dockerCmd, "kill", containerName).Run()
					exec.Command(*dockerCmd, "rm", containerName).Run()
					log.Infoln("Starting container (configuration changed)")
					exec.Command(*dockerCmd, launchCommands...).Run()
				} else {
					log.Debugln("Container launch config is identical: taking no action")
				}
			}
		}
	}
}

// Queries down the chain of possible hostnames and returns a map of docker
// containers we need to query for configuration.
func containerRecords(prefix string, hostname string, canInherit bool) map[string]interface{} {
	// Holds the deduplicated set of containers this host has config for
	containers := make(map[string]interface{})

	// Split the hostname up into fragments
	hostParts := strings.Split(hostname, ".")

	for idx, _ := range hostParts {
		// Calculate the fragment
		domain := strings.Join(hostParts[idx:], ".")
		// Determine the full DNS name with the config prefix
		dnsName := prefix + "." + domain

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

	return containers
}

// Queries down the chain of possible hostnames and returns the container launch
// configuration as specified by DNS. Container config is single level only
// (i.e. super-domains do not override or add to subdomains, regardless of
// inheritance rules)
func containerCommands(containerName string, prefix string, hostname string) string {
	// Split the hostname up into fragments
	hostParts := strings.Split(hostname, ".")

	for idx, _ := range hostParts {
		// Calculate the fragment
		domain := strings.Join(hostParts[idx:], ".")
		// Determine the full DNS name with the config prefix
		dnsName := containerName + "." + prefix + "." + domain

		txt, err := net.LookupTXT(dnsName)
		if err != nil {
			log.Debugln("Failed querying", dnsName, err)
		} else {
			log.Debugln("Lookup", dnsName, "found config", txt[0])
			return txt[0]
		}
	}

	log.Infoln("Container launch disabled by missing config")
	return ""
}