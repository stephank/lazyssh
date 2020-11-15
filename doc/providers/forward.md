# Dummy forwarding target type

The `forward` target type does not actually create any resources, but simply
forwards the connection to a fixed address.

These are the available target options:

```hcl
target "<address>" "forward" {

  # The address to forward connections to. (Required)
  to = "example.com"

}
```
