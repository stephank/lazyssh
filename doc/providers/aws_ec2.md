# AWS EC2 target type

The `aws_ec2` target type uses the AWS SDK to launch (and eventually terminate)
a single EC2 instance.

The AWS SDK looks for configuration in the same place as the AWS CLI, so you
may follow the configuration guide for the CLI to setup AWS credentials:
https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html

These are the available target options:

```hcl
target "<address>" "aws_ec2" {

  # The AMI to launch. (Required)
  image_id = "ami-0a25128eec7dbf084"

  # The instance type to launch. (Required)
  instance_type = "t4g.nano"

  # Name of the key pair to launch with. (Required)
  key_name = "example"

  # Optional subnet ID to launch the instance in.
  subnet_id = "subnet-00000000000000000"

  # Optional user data to provide to the instance. The contents of this will be
  # base64 encoded for you, before it is sent to AWS.
  user_data = <<-EOF
    #cloud-config
    packages: [jq]
  EOF

  # Optional alternate profile to use from local AWS configuration.
  profile = "default"  # The default

  # Optional AWS region to use, if not specified in local AWS configuration.
  region = "eu-west-1"

  # LazySSH waits for this TCP port to be open before forwarding connections to
  # the EC2 instance.
  check_port = 22  # The default

  # Whether to share the instance when LazySSH receives multiple SSH
  # connections. This is the default, and when setting this to false
  # explicitely, LazySSH will launch a unique instance for every SSH
  # connection.
  shared = true  # The default

  # When shared is true, this is the amount of time the EC2 instance will
  # linger before it is terminated. The default is to terminate the instance
  # immediately when the last connection is closed.
  linger = "0s"  # The default

  # Optional EBS volume configuration. This block can be repeated multiple
  # times to configure several devices.
  #
  # A common use case is to set the size of the root volume. To do so, use the
  # device name of the root volume from the AMI, and set just `volume_size`.
  ebs_block_device {

    # Name of the EBS volume. (Required)
    device_name = "/dev/xvda"

    # Whether to delete the volume on instance termination.
    delete_on_termination = true

    # Whether to encrypt the volume.
    encrypted = false

    # KMS key ID used to encrypt the volume.
    kms_key_id = "00000000-0000-0000-0000-000000000000"

    # When set, creates the volume from the given snapshot.
    snapshot_id = "snap-00000000000000000"

    # Size in GiB.
    volume_size = 40

    # Type of volume. One of: standard, gp2, io1, io2
    volume_type = "gp2"

    # Provisioned IOPS. Required for volume types `io1` and `io2`.
    iops = 400

  }

}
```
