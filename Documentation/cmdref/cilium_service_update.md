<!-- This file was autogenerated via cilium cmdref, do not edit manually-->

## cilium service update

Update a service

### Synopsis

Update a service

```
cilium service update [flags]
```

### Options

```
      --backends strings   Backend address or addresses (<IP:Port>)
      --frontend string    Frontend address
  -h, --help               help for update
      --id uint            Identifier
      --k8s-external       Set service as a k8s ExternalIPs
      --k8s-node-port      Set service as a k8s NodePort
```

### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.cilium.yaml)
  -D, --debug           Enable debug messages
  -H, --host string     URI to server-side API
```

### SEE ALSO

* [cilium service](../cilium_service)	 - Manage services & loadbalancers

