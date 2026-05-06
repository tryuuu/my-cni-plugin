# my-cni-plugin

A simple CNI plugin for a home Kubernetes cluster.

## Build

```bash
GOOS=linux GOARCH=amd64 go build -o bin/my-cni ./cmd/my-cni/
```

## Deploy to a Node

Transfer the binary to the node:

```bash
scp bin/my-cni <user>@<node>:/tmp/my-cni
```

Install the binary:

```bash
ssh <user>@<node>
sudo mv /tmp/my-cni /opt/cni/bin/my-cni
sudo chmod +x /opt/cni/bin/my-cni
```

Place the CNI config (set `subnet` and `gateway` per node):

```bash
sudo tee /etc/cni/net.d/10-my-cni.conf <<'EOF'
{
  "cniVersion": "1.0.0",
  "name": "my-cni",
  "type": "my-cni",
  "bridgeName": "cni0",
  "mtu": 1500,
  "ipam": {
    "type": "host-local",
    "subnet": "<node-pod-cidr>",
    "gateway": "<node-pod-gateway>"
  }
}
EOF
```
