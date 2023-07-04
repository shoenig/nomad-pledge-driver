job "passwd" {
  type = "sysbatch"

  group "group" {
    task "cat" {
      driver = "pledge"
      user   = "root"
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
