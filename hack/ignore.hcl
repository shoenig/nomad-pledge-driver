job "ignore" {
  group "group" {
    task "ignore" {
      driver       = "pledge"
      kill_timeout = "5s"
      config {
        command  = "python3"
        args     = ["${NOMAD_TASK_DIR}/ignore.py"]
        promises = "stdio rpath tty"
        unveil   = ["r:/${NOMAD_TASK_DIR}"]
      }

      template {
        destination = "local/ignore.py"
        data        = <<EOH
import signal
import os
import time

if __name__ == '__main__':
    signal.signal(signal.SIGINT, signal.SIG_IGN)
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    signal.pause()
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
