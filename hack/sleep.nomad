job "sleep" {
  datacenters = ["dc1"]
  type        = "batch"

  group "group" {
    task "sleep" {
      driver = "pledge"
      config {
        command    = "sleep"
        args       = ["1d"]
        promises   = "stdio rpath"
        importance = "highest"
      }
    }
  }
}
