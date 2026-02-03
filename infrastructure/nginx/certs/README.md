# Self-Signed Certificates for Internal Nginx

These certificates are used for internal communication between Caddy and nginx.
Caddy terminates public TLS and proxies to nginx over HTTPS with certificate verification disabled.

## Generate certificates

```bash
openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
  -keyout server.key \
  -out server.crt \
  -subj "/CN=northcloud.biz/O=NorthCloud/C=CA" \
  -addext "subjectAltName=DNS:northcloud.biz,DNS:localhost"
```

The certificates don't need to be trusted since Caddy uses `tls_insecure_skip_verify`.
