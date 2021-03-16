# STS-Mate
An [MTA-STS](https://tools.ietf.org/html/draft-ietf-uta-mta-sts) policy
server/reverse proxy. Uses LetsEncrypt to fetch certs for your host.

# I just want to set up MTA-STS for my GSuite domain

*Update:* I've stopped hosting a "catchall" reverse proxy for GSuite, because
nobody else seemed to be using it.

Instead, I've deployed sts-mate on Google Cloud Run, which is slightly cheaper
but does not allow dynamically fetching TLS certs. See instructions below on how
to deploy your own.

~~You're in luck! With just two simple DNS records, you can set up MTA-STS on your
GSuite-hosted domain.~~

```
_mta-sts.[yourdomain] CNAME _mta.sts.google.com.
_mta-sts.[yourdomain] CNAME gsuite-mta-sts.af0.net.
```

~~An example configuration can be found at `af0.net`.~~

~~(Note: I can't promise any ongoing support for this service, so use at your own
risk, but STS fails gracefully--cached policies expire and fail open.)~~

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
_mta-sts.yourdomain.     300     IN      CNAME   _mta-sts.af0.net.
mta-sts.yourdomain.      300     IN      CNAME   mta-sts.af0.net.
```

Presto: magic MTA-STS. (You can test it with [sts-tester.af0.net/](http://sts-tester.af0.net/).)

# Installing via Docker

Run these commands to install in a Docker container. This will download and build from Github
and configure the container to run on the host's port 443.

```
$ docker build github.com/danmarg/sts-mate -t mta-sts
$ docker run -dit --restart unless-stopped --net host mta-sts
```

# Deploying on Google Cloud Run

These instructions require the [gcloud](https://cloud.google.com/sdk/gcloud/)
tool.

```
$ export PROJECT="my project ID"  # This is just a unique ID
$ export DOMAIN="example.user"  # Your domain
$ gcloud projects create $PROJECT
$ git clone https://github.com/danmarg/sts-mate.git
$ cd sts-mate
$ gcloud builds submit --tag gcr.io/$PROJECT/sts-mate  # Submit a Docker image
$ gcloud run deploy \   # Deploy on Cloud Run
   --image gcr.io/$PROJECT/sts-mate \
  --platform managed \
  --update-env-vars STS_DOMAIN="$DOMAIN",HTTP="http",MIRROR_STS_FROM="google.com"
$ gcloud domains verify $DOMAIN   $ Verify you own your domain
$ gcloud beta run domain-mappings create \  # Map your domain to Cloud Run
  --service sts-mate --domain mta-sts.$DOMAIN
```

You will have to update your domain's DNS with the requisite CNAME (per the
output of the last command above) and, of course, the `_mta-sts` TXT record.

