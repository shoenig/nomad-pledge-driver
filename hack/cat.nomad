job "cat" {
  datacenters = ["dc1"]
  type        = "sysbatch"

  group "group" {
    task "cat" {
      driver = "pledge"
      config {
        command = "cat"
        args    = ["/etc/passwd"]
        pledges = "stdio rpath"
      }
    }
  }
}
