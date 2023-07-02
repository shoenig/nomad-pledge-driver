# nomad-pledge-driver

![GitHub](https://img.shields.io/github/license/shoenig/nomad-pledge-driver?style=flat-square)
[![Run E2E Tests](https://github.com/shoenig/nomad-pledge-driver/actions/workflows/e2e.yaml/badge.svg)](https://github.com/shoenig/nomad-pledge-driver/actions/workflows/e2e.yaml)

`nomad-pledge-driver` is a Nomad task driver based on the `pledge` [utility for Linux](https://justine.lol/pledge/) by Justine Tunney.

*Security through SECCOMP sorcery*

#### :warning: Experimental :warning:

This task driver is experimental. It might work, it might crash, or it might do nothing at all.
Changes to the driver implementation and how it is configured should be expected.
Use with caution.

### Features

- Sandbox applications by **restricting syscalls** they are able to make (via _promises_)
- Sandbox applications by **allow-listing filepaths** they are allowed to access (via _unveil_)
- Sandbox applications by **isolating processes** using modern Linux cgroups (via _cgroups v2_)

### Examples

The example below uses `curl` to fetch `example.com`, with the minimal set of promises to make a request.

More complex examples in the [hack](hack) directory.

```hcl
job "curl" {
  type        = "batch"

  group "group" {
    task "curl" {
      driver = "pledge"
      user   = "nobody"
      config {
        command  = "curl"
        args     = ["example.com"]
        promises = "stdio rpath inet dns sendfd"
        unveil     = ["r:${NOMAD_TASK_DIR}"]
      }
    }
  }
}
```

### Building

The `nomad-pledge-driver` plugin is written in Go. It can be built using the normal Go toolchain steps, but
the Makefile contains a `dev` target to make things easy. The compiled binary will appear in the `output/`
directory.

```shell
make dev
```

### Installing 

The plugin should be placed in the `plugin_dir` configured by the Nomad agent, per Nomad's [documentation](https://www.nomadproject.io/docs/configuration#plugin_dir).

You'll also need the `pledge` executable (1.8 or higher) that powers the plugin sandboxing.
Download the `pledge` executable from https://justine.lol/pledge/ and install it somewhere.
The plugin configuration lets you specify where the path to the pledge executable.

```shell
sudo mkdir -p /opt/bin
curl -L -o /opt/bin/pledge-1.8.com https://justine.lol/pledge/pledge-1.8.com
```

:point_right: **optional** It is very convenient to bless the pledge executable with the `cap_net_bind_service`
Linux capability. This will enable Nomad tasks using the pledge driver to bind to privileged
ports (e.g. below 1024).

```shell
sudo setcap cap_net_bind_service+eip /opt/bin/pledge-1.8.com
```

The plugin will expose the `driver.pledge.cap.net_bind` attribute indicating whether
the `cap_net_bind_service` capability has been set on the `pledge-1.x.com` executable.

### Plugin Configuration

Currently there is only one configuration option for this plugin, which is to specify the path of the `pledge` executable.

```hcl
plugin "pledge" {
  config {
    pledge_executable = "/opt/bin/pledge-1.8.com"
  }
}
```

Note: in these examples the driver plugin is named `pledge`, and the utility executable is named `pledge-1.6.com`. 

### Task Configuration

Tasks need to specify which **promises** they require in order to run.

Tasks also need to **unveil** the filesystem paths needed to run.

For more information about which pledges are available and how this mechanism works, visit https://justine.lol/pledge/

If no `user` is specified for the task, the pledge plugin will use the `nobody` user by default.

- `command`: The executable to run
- `args`: The arguments to pass to executable
- `promises`: The set of promises needed for the executable to run
- `unveil`: The set of system filepaths to allow the task to access, and with what permission
- `importance`: One of `lowest`, `low`, `normal`, `high`, `highest` (default is `normal`)

```hcl
# see hack/http.nomad for complete python http.server example
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
```

### Troubleshooting

For help getting the plugin to work, see the [TROUBLESHOOT](TROUBLESHOOT.md) doc.
Otherwise feel free to file an issue!

### Contributing

The `nomad-pledge-driver` plugin is currently under active development - anything may change at a moments notice!

#### hacking

The included Makefile includes helpful targets for hacking on the pledge plugin.

To simply compile, run `make dev`. The output will go into `/tmp/plugins`.

To start Nomad with the plugin, run `make run`. Under the hood this is using the `hack/client.hcl` Client
config file, along with `-dev` mode defaults. You should be able to run jobs making use of `pledge` driver
when launching Nomad this way.

There are example jobs in the [hack/](hack) directory.

### License

The `noamd-pledge-driver` task driver plugin is made open source under the [MPL-2.0](LICENSE) license.
