job "curl" {
  datacenters = ["dc1"]
  type        = "sysbatch"

  group "group" {
    task "curl" {
      driver = "pledge"
      user   = "nobody"
      config {
        command  = "curl"
        args     = ["example.com"]
        promises = "stdio rpath inet unix sendfd dns"
      }
    }
  }
}
