# To be used in GHA with e2e setup.

data_dir   = "/opt/nomad/data"
bind_addr  = "0.0.0.0"
plugin_dir = "/tmp/plugins"

server {
  enabled          = true
  bootstrap_expect = 1

  default_scheduler_config {
    memory_oversubscription_enabled = true
  }
}

client {
  enabled = true
  servers = ["127.0.0.1"]
}


plugin "nomad-pledge-driver" {
  config {
    pledge_executable = "/opt/bin/pledge-1.8.com"
  }
}

