job "curl" {
  datacenters = ["dc1"]
  type        = "batch"

  group "group" {
    task "curl" {
      driver = "pledge"
      user   = "nobody"
      config {
        command  = "curl"
        args     = ["example.com"]
        promises = "stdio rpath inet dns sendfd"
      }
    }
  }
}
