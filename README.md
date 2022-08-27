# nomad-pledge-driver

![GitHub](https://img.shields.io/github/license/shoenig/nomad-pledge-driver?style=flat-square)

`nomad-pledge-driver` is a Nomad task driver based on the `pledge` [utility for Linux](https://justine.lol/pledge/) by Justine Tunney.

### :warning: Experimental :warning:

This task driver is experimental. It might work, it might crash, or it might do nothing at all.
Changes to the driver implementation and how it is configured should be expected.
Use with caution, and certainly not in any mission-critical Nomad jobs.

### Features

- Sandbox applications by *restricting syscalls* they are able to make.

### Examples

This example uses `curl` to fetch `example.com`, with the minimal set of promises for making the request and outputing to standard out.

```hcl
job "curl" {
  datacenters = ["dc1"]
  type        = "sysbatch"

  group "group" {
    task "curl" {
      driver = "pledge"
      user   = "nobody"
      config {
        command = "curl"
        args    = ["example.com"]
        pledges = "stdio rpath inet unix sendfd dns"
      }
    }
  }
}
```

### Building

The `nomad-pledge-driver` plugin is written in Go. It can be built using the normal Go toolchain steps, e.g.

```shell
go build
```

### Installing 

The plugin should be placed in the `plugin_dir` configured by the Nomad agent, per Nomad's [documentation](https://www.nomadproject.io/docs/configuration#plugin_dir).

You'll also need the `pledge` executable that powers the plugin sandboxing. Download the `pledge` executable from https://justine.lol/pledge/ and install it somewhere. The plugin configuration lets you specify where the path to the pledge executable.

### Plugin Configuration

Currently there is only one configuration option for this plugin, which is to specify the path of the `pledge` executable.

```hcl
plugin "pledge" {
  config {
    pledge_executable = "/opt/bin/pledge-1.6.com"
  }
}
```

Note: in these examples the driver plugin is named `pledge`, and the utility executable is named `pledge-1.6.com`. 

### Task Configuration

Tasks need to specify which **pledges** they require in order to run.

For more information about which pledges are available and how this mechanism works, visit https://justine.lol/pledge/

If no `user` is specified for the task, the pledge plugin will use the `nobody` user by default.

- `command`: The executable to run
- `args`: The arguments to pass to executable
- `pledges`: The set of pledges needed for the executable to run

```hcl
task "curl" {
  driver = "pledge"
  config {
    command = "curl"
    args    = ["example.com"]
    pledges = "stdio rpath inet unix sendfd dns"
  }
}
```

### Contributing

The `nomad-pledge-driver` plugin is currently under active development - anything may change at a moments notice!

### License

The `noamd-pledge-driver` task driver plugin is made open source under the [MPL-2.0](LICENSE) license.
