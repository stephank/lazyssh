# LazySSH

LazySSH is an SSH server that acts as a jump host only, and dynamically starts
temporary virtual machines.

If you find yourself briefly starting a virtual machine just to SSH into it and
try something out, LazySSH is an attempt to automate that flow via just the
`ssh` command. LazySSH starts the machine for you when you connect, and shuts
it down (some time after) you disconnect.

Another possible use is to have LazySSH sit in front of a build server to start
specific types of machines for your build. (Think different CPU architectures
or operating systems.)

**Important**: LazySSH is a young piece of code. If you're going to use it to
create resources that cost money (like AWS EC2 instances), keep a close eye on
usage. If, for example, you put your laptop to sleep at the wrong time, or
LazySSH crashes for whatever reason, it may leave resources running.

**Important**: The security of LazySSH has not been tested in any way, so it's
probably best to run it in a closed setting. (Not facing the public internet or
otherwise firewalled.) The SSH server implementation is based on
[golang.org/x/crypto].

License: AGPL v3

[golang.org/x/crypto]: https://pkg.go.dev/golang.org/x/crypto

## Usage

There are several ways to get LazySSH:

- Grab a binary from the [releases page].

- Docker images are available on Docker Hub as
  [stephank/lazyssh](https://hub.docker.com/r/stephank/lazyssh).

- Nix users, whether you use flakes or not, see the documentation in
  [flake.nix](./flake.nix).

- If you instead want to build LazySSH yourself, you need at least Go 1.13,
  then just `go build`.

[releases page]: https://github.com/stephank/lazyssh/releases

You need to generate an SSH host key and client key. The host key is what the
server uses to identify itself, while the client key is what you connect with.

```sh
# Both of these also generate a .pub file with the public half of the key pair.
ssh-keygen -t ed25519 -f lazyssh_host_key
ssh-keygen -t ed25519 -f lazyssh_client_key
```

Now create a `config.hcl` file that looks like:

```hcl
server {
  # Set this to the contents of lazyssh_host_key generated above.
  host_key = <<-EOF
    -----BEGIN OPENSSH PRIVATE KEY-----
    [...]
    -----END OPENSSH PRIVATE KEY-----
  EOF

  # Set this to the contents of lazyssh_client_key.pub generated above.
  authorized_key = <<-EOF
    ssh-ed25519 [...]
  EOF
}
```

The `server` block is followed by one or more `target` blocks. Here are the
types of targets currently supported, and links to the documentation:

- [AWS EC2](./doc/providers/aws_ec2.md)
- [VirtualBox](./doc/providers/virtualbox.md)
- [Dummy forwarding](./doc/providers/forward.md)

Once your config is ready, you can start the server:

```sh
./lazyssh -config ./config.hcl
```

> Using Docker? You can start the container with, for example:
>
> ```sh
> docker run \
>   -p 7922:7922 \
>   -v /path/to/config.hcl:/config.hcl:ro \
>   stephank/lazyssh
> ```

You usually need an entry for LazySSH in your `~/.ssh/config`, because the
`ssh` command otherwise doesn't make all options available for jump-hosts. Here
is a sample config:

```
Host lazyssh
  Hostname localhost
  Port 7922
  User jump
  PreferredAuthentications publickey
  IdentityFile ~/path/to/lazyssh_client_key
  IdentitiesOnly yes
```

Now you should be ready to go:

```sh
ssh -J lazyssh user@mytarget
```

For more details, see [the included documentation](./doc/index.md).
