# VOX3 Exporter

Prometheus exporter for Vodafone VOX 3 (and presumably other THG3000 variants) DSL modem statistics

## Configuration

1. Set environment variables:
```sh
VOX3_IP=ip_address # Optional: defaults to 192.168.1.1
VOX3_PASSWORD=password # Required
```

## Docker (Compose)

```yaml
version: '3'
services:
  vox3-exporter:
    image: ghcr.io/njallam/vox3_exporter
    container_name: vox3_exporter
    restart: unless-stopped
    ports:
      - "9917:9917"
    environment:
      - VOX3_PASSWORD=password
```
