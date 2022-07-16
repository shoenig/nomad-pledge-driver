job "raw_env" {
  datacenters = ["dc1"]
  type        = "sysbatch"

  group "group" {
    task "printenv" {
      driver = "pledge"
      user   = "nobody"
      config {
        command = "env"
      }
    }
  }
}
