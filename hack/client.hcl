client {
  enabled = true
}

plugin_dir = "/home/shoenig/Work/go/pledge-d/output"

plugin "pledge" {
  config {
    pledge_executable = "/opt/bin/pledge-1.7.com"

    #   namespace "*" {
    #     allow_pledges = "stdio rpath"
    #   }

    #   namespace "cache" {
    #     allow_pledges = "stdio rpath inet proc thread"
    #   }

    #   namespace "databases" {
    #     allow_pledges = "stdio rpath wpath cpath dpath flock inet dns proc thread"
    #   }

    #   namespace "admin" {
    #     allow_pledges = "*"
    #   }
  }
}

# stdio: allow stdio and benign system calls
# rpath: read-only path ops
# wpath: write path ops
# cpath: create path ops
# dpath: create special files
# flock: file locks
# tty: terminal ioctls
# recvfd: allow SCM_RIGHTS
# fattr: allow changing some struct stat bits
# inet: allow IPv4 and IPv6
# unix: allow local sockets
# dns: allow dns
# proc: allow fork, clone and friends
# thread: allow clone
# id: allow setuid and friends
# exec: allow executing ape binaries
