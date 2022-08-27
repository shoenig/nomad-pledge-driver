job "env" {
  datacenters = ["dc1"]
  type        = "sysbatch"

  group "group" {

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "env" {
      driver = "pledge"
      user   = "anonymous"
      config {
        command  = "env"
        args     = []
        promises = "stdio rpath"
      }
    }
  }
}
