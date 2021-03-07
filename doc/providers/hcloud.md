# Hetzner Cloud target type

The `hcloud` target type uses the HCloud SDK to launch and terminate a single hcloud server.

These are the available target options:

```hcl
target "<address>" "hcloud" {

  # The API token to use. (Required)
  token = "9vx8w..."

  # The image to launch. (Required)
  image = "ubuntu-20.03"

  # The server type to launch. (Required)
  server_type = "cx11"

  # Name of the key pair to launch with. (Required)
  ssh_key = "my-keypair"

  # Name of the location to launch server in. (Required)
  location = "nbg1"

  # Optional user data to provide to the instance.
  user_data = <<-EOF
    #cloud-config
    packages: [jq]
  EOF

  # LazySSH waits for this TCP port to be open before forwarding connections to
  # the hcloud server.
  check_port = 22  # The default

  # Whether to share the server when LazySSH receives multiple SSH
  # connections. This is the default, and when setting this to false
  # explicitely, LazySSH will launch a unique instance for every SSH
  # connection.
  shared = true  # The default

  # When shared is true, this is the amount of time the EC2 instance will
  # linger before it is terminated. The default is to terminate the instance
  # immediately when the last connection is closed.
  linger = "0s"  # The default

}
```
