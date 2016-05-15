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