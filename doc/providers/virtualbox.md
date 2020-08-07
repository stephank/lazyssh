# VirtualBox target type

The `virtualbox` target type starts and stops [VirtualBox] virtual machines
by automating calls to the `VBoxManage` command-line tool.

These are the available target options:

```hcl
target "<address>" "virtualbox" {

  # Name of the virtual machine to manage. (Required)
  # This may also be the UUID of the machine.
  name = "Debian"

  # Address where the machine is available. (Required)
  # If you rely on port-forwarding, you may want to set this to 'localhost'.
  addr = "192.168.0.100"

  # LazySSH waits for this TCP port to be open before forwarding connections to
  # the above address.
  check_port = 22  # The default

  # Which type of startup to request.
  # Valid values: gui, headless, separate
  start_mode = "headless"  # The default

  # Which type of shutdown to request.
  # Valid values: poweroff, acpipowerbutton, acpisleepbutton
  stop_mode = "acpipowerbutton"  # The default

  # The amount of time the virtual machine will linger before it is stopped.
  # The default is to stop the instance immediately when the last connection is
  # closed.
  linger = "0s"  # The default

}
```
