# Docker DNS Provisioning

This is a small utility which does a differential provisioning of
docker containers on a host based on the contents of DNS TXT records.

It is designed to solve the bootstrap problem of trying to provision an
otherwise immutable host that needs to do some initial stand up of more
variable but brittle services, namely KV-stores (like etcd/zookeeper)
and Mesos slaves which depend on them.

While there are certainly other approaches to this problem, there is a
degree of simplicity implied in simply using the other service you have
to get right - naming and DNS - to supply this information.

Currently this is command line focused. In future it may move to using
full JSON-container launch commands.

## Example DNS configuration with dnsmasq

We can configure to launch a set of containers like so:
```
txt-record=containers.docker.will-desktop,etcd
txt-record=containers.docker.will-desktop,bash-sleeper
txt-record=containers.docker.will-desktop,bash-echoer

txt-record=etcd.containers.docker.will-desktop,"quay.io/coreos/etcd"
txt-record=bash-sleeper.containers.docker.will-desktop,"ubuntu:wily sleep 60"
txt-record=bash-echoerr.containers.docker.will-desktop,"ubuntu:wily echo HelloDNS!"
```

Obviously this isn't very useful - the intended purpose of this is to
do something more useful like launch that etcd process with some
discovery options (say, DNS-based)
```
txt-record=etcd.containers.docker.will-desktop,"quay.io/coreos/etcd --discovery-srv will-desktop --initial-advertise-peer-urls http://will-desktop:2380 --initial-cluster-token will-desktop-cluster-1 --initial-cluster-state new --advertise-client-urls http://will-desktop:2379 --listen-client-urls http://will-desktop:2379 --listen-peer-urls http://will-desktop:2380"
```

## Internal management
Container names are king - if a name is specified in DNS, it is assumed
we are to take control of it's configuration. Beyond that, management
of additional containers is based off docker labels - the label
`docker-dns-provision.command` is treated as a marker for whether
extraneous containers should be removed.

# Work-in-progress
This is a proof-of-concept (hence the use of command lines). A likely
future change is moving away from being name-specific and using docker
container labels only to determine our control plane, as this is much
less intrusive.