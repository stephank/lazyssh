# LazySSH documentation

LazySSH uses [HCL] as its configuration format. By default it'll try to read
`config.hcl` in the current working directory, but a different file can be
specified using command-line arguments:

```sh
lazyssh -config ./filename.hcl
```

[hcl]: https://pkg.go.dev/github.com/hashicorp/hcl/v2@v2.7.0

## Main server configuration

The SSH server itself is configured with the `server` block. The following
example HCL lists all available options:

```hcl
server {

  # The address the server will listen on.
  listen = "localhost:7922"  # The default

  # The SSH host key the server uses to identify itself. (Required)
  host_key = <<-EOF
    -----BEGIN OPENSSH PRIVATE KEY-----
    [...]
    -----END OPENSSH PRIVATE KEY-----
  EOF

  # A single SSH public key the client uses to identify itself. (Required)
  authorized_key = <<-EOF
    ssh-ed25519 [...]
  EOF

}
```

## Target configuration

The config file further contains any number of `target` blocks. These are all
in the format:

```hcl
target "<address>" "<type>" {

  # Type-specific settings

}
```

Where `<address>` is the virtual address the SSH client can connect to through
this jump-host, and `<type>` is one of the supported target types by LazySSH.

Target types and their settings are documented separately:

- [AWS EC2](./providers/aws_ec2.md)
- [VirtualBox](./providers/virtualbox.md)
- [Dummy forwarding](./providers/forward.md)
