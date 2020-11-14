# LazySSH to-do

This file lists a bunch of things the original author (stephank) thought would
be good improvements.

I'm not necessarily working on these. If you want to pick something up, pull
requests are welcome, of course. If you'd like to indicate you're working on
something to avoid conflict, create an issue for it.

## General

- A code review by someone more experienced in Go would be appreciated. I'm not
  sure how this would work, but I'm happy to discuss it in issues (Or if you're
  bold, a pull request.)

- Launchd agent plist

- Systemd service unit

- Socket activation

- Multiple authorized keys.

- Persist state so any kind of interruption can recover management of an
  instance. (We'd still interrupt all connections, but can hopefully prevent
  accidental waste of resources this way.)

- Figure out some way to provide meaningful feedback to clients while doing
  work. This appears to be a difficult problem, because the OpenSSH client
  doesn't print debug messages sent by the server unless using `-v`. The only
  other opportunity appears to be the pre-auth banner, which is not useful for
  us. Maybe someone else has a clever idea?

- Figure out ways to cleanly interrupt machine startup. Maybe this is a
  per-provider thing.

- There may be additional `TODO` comments in code.

## More providers

- Google Cloud Compute

- DigitalOcean Droplets

- Hetzner Cloud

- Scaleway

- Vultr

- Others?

- It'd be interesting if there was some generic (but still friendly) way we
  could bridge with Terraform providers or Packer builders. I haven't looked
  into it, because it didn't seem useful to spend time on, given the very basic
  requirements I started out with.

## AWS EC2

- Connect more `RunInstances` options to config.

- Some way to select an AMI based on filter criteria, like Terraform and Packer
  allow. (ie. 'Automatically select the _latest_ Debian AMI')

- Maybe add support for spot instances? I've never worked with them.

- Optionally help with connectivity by creating a security group for the user.

## VirtualBox

- Create new temporary machines from an OVA.
