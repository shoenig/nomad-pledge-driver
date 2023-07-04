job "bridge" {
  group "group" {
    network {
      mode = "bridge"
      port "http" { to = 8181 }
    }

    service {
      provider = "nomad"
      name     = "pybridge"
      port     = "http"
      check {
        name     = "up"
        type     = "http"
        path     = "/index.html"
        interval = "6s"
        timeout  = "1s"
      }
    }

    task "python" {
      driver = "pledge"
      user   = "nobody"
      config {
        command  = "python3"
        args     = ["-m", "http.server", "8181", "--directory", "${NOMAD_TASK_DIR}"]
        promises = "stdio rpath inet"
        unveil   = ["r:/etc/mime.types", "r:${NOMAD_TASK_DIR}"]
      }

      template {
        destination = "local/index.html"
        data        = <<EOH
<!doctype html>
<html>
  <title>bridge mode</title>
  <body><p>Hello, pal!</p></body>
</html>
EOH
      }
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    update {
      min_healthy_time = "2s"
    }
  }
}
