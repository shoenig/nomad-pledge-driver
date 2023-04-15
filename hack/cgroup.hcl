job "cgroup" {
  type = "sysbatch"

  group "group" {
    task "cat" {
      driver = "pledge"
      config {
        command  = "/bin/cat"
        args     = ["/proc/self/cgroup"]
        promises = "stdio rpath"
        unveil   = ["r:/proc/self/cgroup"]
      }
    }
  }
}
