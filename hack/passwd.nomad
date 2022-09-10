job "passwd" {
  datacenters = ["dc1"]
  type        = "sysbatch"

  group "group" {
    task "cat" {
      driver = "pledge"
      config {
        command  = "cat"
        args     = ["/etc/passwd"]
        promises = "stdio rpath"
      }
    }

    restart {
      attempts = 0
      mode     = "fail"
    }
  }
}
