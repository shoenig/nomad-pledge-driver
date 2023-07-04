job "env" {
  type = "batch"

  group "group" {
    task "env" {
      driver = "pledge"
      config {
        command  = "env"
        promises = "stdio rpath"
      }
    }
  }
}
