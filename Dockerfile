FROM golang:latest
LABEL maintainer="Daniel Margolis <dan@af0.net>"
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o sts-mate .
EXPOSE 443

# Comma-separated list of domains to serve policies for.
ENV STS_DOMAIN "example.user"
# Source of the live STS policy to mirror.
ENV MIRROR_STS_FROM "google.com"
# If set, to "http", serves the policy on HTTP (instead of HTTPS). Good for
# when you are behind an HTTPS-terminating reverse proxy.
ENV HTTP "nohttp"
# Usage:
#  --domain is the domain to serve a policy for.
#  --mirror_sts_from is the mail domain from which to proxy STS policies
#  --domain is the domain for which to serve a policy (if limited)
CMD ["sh", "-c", "./sts-mate --domain $STS_DOMAIN --mirror_sts_from $MIRROR_STS_FROM --$HTTP"]
