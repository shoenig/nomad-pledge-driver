job "http" {
  datacenters = ["dc1"]
  type        = "service"

  group "group" {
    network {
      mode = "host"
      port "http" { static = "8181" }
    }

    task "task" {
      driver = "pledge"
      user   = "nobody"
      config {
        command    = "python3"
        args       = ["-m", "http.server", "${NOMAD_PORT_http}", "--directory", "${NOMAD_TASK_DIR}"]
        promises   = "stdio rpath inet"
        unveil     = ["r:/etc/mime.types", "r:${NOMAD_TASK_DIR}"]
        importance = "low"
      }

      template {
        destination = "local/index.html"
        data        = <<EOH
<!doctype html>
<html>
  <title>example</title>
  <body><p>Hello, friend!</p></body>
</html>
EOH
      }
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    update {
      min_healthy_time = "1s"
    }
  }
}
