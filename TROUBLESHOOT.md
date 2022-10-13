# Troubleshooting

### Symptom: `pledge executable not found`

```
Driver       Detected  Healthy  Message                      Time
pledge       false     false    pledge executable not found  2022-10-13T09:24:52-05:00
```

#### Possible cause: pledge_executable not set correctly

Be sure `pledge_executable` is configured to point to the pledge-1.x.com executable downloaded from https://justine.lol/pledge/

### Symptom: `pledge binary is not executable`

```
Driver       Detected  Healthy  Message                          Time
pledge       true      false    pledge binary is not executable  2022-10-13T09:30:38-05:00
```

#### Possible cause: pledge binary is not executable

Be sure the pledge-1.x.com binary is set to be executable.

```
chmod +x pledge-1.x.com
```

### Symptom: `kernel landlock not enabled`

```
Driver       Detected  Healthy  Message                      Time
pledge       true      false    kernel landlock not enabled  2022-10-13T08:53:35-05:00
```

#### Possible cause: kernel not compiled with landlock enabled

https://docs.kernel.org/userspace-api/landlock.html

Mostly this means using a Linux distribution that ships with landlock enabled.

Ubuntu 22.04 LTS is a known-good distribution.

If landlock is enabled, it will show up in `dmesg`, e.g.

```
➜ sudo dmesg | grep landlock
[    0.471728] landlock: Up and running.
```

### Symptom: `The argument "pledge_executable" is required, but no definition was found.`

Full error:

```
==> Error starting agent: failed to create plugin loader: failed to initialize plugin loader: parsing plugin configurations failed: 1 error occurred:
	* plugin "pledge" (driver): failed to parse config:  Missing required argument: The argument "pledge_executable" is required, but no definition was found.
```

#### Possible cause: `pledge_executable` not set in plugin config block

Be sure the plugin is configured in Nomad client config, e.g.

```
plugin "pledge" {
  config {
    pledge_executable = "/opt/bin/pledge-1.8.com"
  }
}
```

#### Possible cause: driver binary name does not match configured plugin name

Be sure the name of the plugin executable matches the name in Client configuration, e.g.

```
plugin "pledge" { ... }
```

must match the actual executable in the Client `plugin_dir`.

```
➜ ls plugins/
pledge
```
