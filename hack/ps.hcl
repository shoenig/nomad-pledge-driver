job "ps" {
  type = "batch"

  group "group" {
    task "ps" {
      driver = "pledge"
      user   = "root"
      config {
        command  = "ps"
        args     = ["aux"]
        promises = "stdio rpath exec"
        unveil   = ["r:/proc"]
      }
    }
  }
}
