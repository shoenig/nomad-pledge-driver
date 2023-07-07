job "resources" {
  type = "batch"

  group "group" {

    task "memory.max" {
      driver = "pledge"
      config {
        command  = "/bin/cat"
        args     = ["/sys/fs/cgroup/nomad.slice/${NOMAD_ALLOC_ID}.${NOMAD_TASK_NAME}.scope/memory.max"]
        promises = "stdio rpath"
        unveil   = ["r:/sys/fs/cgroup/nomad.slice"]
      }
      resources {
        cpu    = 100
        memory = 200
      }
    }

    task "memory.max.oversub" {
      driver = "pledge"
      config {
        command  = "/bin/cat"
        args     = ["/sys/fs/cgroup/nomad.slice/${NOMAD_ALLOC_ID}.${NOMAD_TASK_NAME}.scope/memory.max"]
        promises = "stdio rpath"
        unveil   = ["r:/sys/fs/cgroup/nomad.slice"]
      }
      resources {
        cpu        = 100
        memory     = 150
        memory_max = 250
      }
    }

    task "memory.low.oversub" {
      driver = "pledge"
      config {
        command  = "/bin/cat"
        args     = ["/sys/fs/cgroup/nomad.slice/${NOMAD_ALLOC_ID}.${NOMAD_TASK_NAME}.scope/memory.low"]
        promises = "stdio rpath"
        unveil   = ["r:/sys/fs/cgroup/nomad.slice"]
      }
      resources {
        cpu        = 100
        memory     = 150
        memory_max = 250
      }
    }

    task "cpu.max" {
      driver = "pledge"
      config {
        command  = "/bin/cat"
        args     = ["/sys/fs/cgroup/nomad.slice/${NOMAD_ALLOC_ID}.${NOMAD_TASK_NAME}.scope/cpu.max"]
        promises = "stdio rpath"
        unveil   = ["r:/sys/fs/cgroup/nomad.slice"]
      }
      resources {
        cpu = 1000
      }
    }

    task "cpu.max.cores" {
      driver = "pledge"
      config {
        command  = "/bin/cat"
        args     = ["/sys/fs/cgroup/nomad.slice/${NOMAD_ALLOC_ID}.${NOMAD_TASK_NAME}.scope/cpu.max"]
        promises = "stdio rpath"
        unveil   = ["r:/sys/fs/cgroup/nomad.slice"]
      }
      resources {
        cores = 1
      }
    }
  }
}
