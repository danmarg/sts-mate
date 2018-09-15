# STS-Mate
An [MTA-STS](https://tools.ietf.org/html/draft-ietf-uta-mta-sts) policy server/reverse proxy. Uses LetsEncrypt to fetch certs for
your host.

# Usage

STS-Mate, as opposed to a simple HTTPS server with a plaintext STS policy file,
serves two purposes:

1. Dynamically fetches a LetsEncrypt certificate for your policy host.
2. Reverse-proxies an STS policy from some other (upstream) host.

This means that STS-Mate can be used to serve policies for arbitrary new domains
(if, for example, you run an ISP and want to host policies for all new
customers), and it can also be used to proxy policies from a hosting provider
(say, Google.com).

For example, to serve policies for "example.user":

`sts-mate --domain example.user --sts_mx *.example.user`

To mirror policies from "example.host":

`sts-mate --domain example.user --mirror_sts_from example.host`

To serve policies for anyone pointing a CNAME to `mta-sts.example.host`:

`sts-mate --my_real_host mta-sts.example.host --sts_mx *.example.host`

# Stupid-simple Example

Let's say you host mail for "yourdomain" on Google GSuite. Just create these two DNS records:

```
_mta-sts.yourdomain.       300     IN      CNAME     _mta-sts.af0.net.
mta-sts.yourdomain.      300     IN      CNAME   mta-sts.af0.net.
```

Presto: magic MTA-STS.
