FROM golang:latest
LABEL maintainer="Daniel Margolis <dan@af0.net>"
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o sts-mate .
EXPOSE 443

# Usage:
#  --my_real_host is the hostname running sts-mate
#  --mirror_sts_from is the mail domain from which to proxy STS policies
#  --domain is the domain for which to serve a policy (if limited)
CMD ["./sts-mate --my_real_host mta-sts.af0.net --mirror_sts_from google.com"]
