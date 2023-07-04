job "sleep" {
  group "group" {
    task "sleep" {
      driver = "pledge"
      user   = "nobody"
      config {
        command    = "sleep"
        args       = ["infinity"]
        promises   = "stdio rpath"
        importance = "lowest"
      }
    }
  }
}
