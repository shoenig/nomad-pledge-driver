job "py" {
  datacenters = ["dc1"]
  type        = "service"

  group "group" {

    restart {
      attempts = 0
      mode     = "fail"
    }

    update {
      min_healthy_time = "1s"
    }

    network {
      mode = "host"
      port "http" { static = "8800" }
    }

    task "pyhttp" {
      driver = "pledge"
      user   = "nobody"
      config {
        command  = "python3"
        args     = ["-m", "http.server", "${NOMAD_PORT_http}", "--directory", "/tmp"]
        promises = "stdio rpath inet"
        unveil   = ["r:/etc", "r:/tmp"]
      }
    }
  }
}
