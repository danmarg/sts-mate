package main

import (
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	certsDir = "certs"
	hostsDir = "hosts"
)

var (
	domains           = flag.String("domain", "", "Domain(s) for which to serve policy (comma-separated). If empty, any domain is allowed.")
	certdir           = flag.String("certificate_dir", "certificate-dir", "Directory in which to store certificates.")
	myRealHost        = flag.String("my_real_host", "", "If set, ensure that any host we haven't seen has a CNAME to us.")
	tryCertNoMoreThan = flag.Duration("try_cert_no_more_often_than", 24*time.Hour, "Don't try to request a cert for a host more often than this.")
	serveHTTP         = flag.Bool("http", false, "If true, serves HTTP instead of HTTPS (and does not fetch certs). Useful for serving behind an HTTPS-terminating proxy.")
	staging           = flag.Bool("staging", false, "If true, uses Let's Encrypt 'staging' environment instead of prod.")
	acmeEndpoint      = flag.String("acme_endpoint", "", "If set, uses a custom ACME endpoint URL. It doesn't make sense to use this with --staging.")

	// Policy options.
	mirrorStsFrom = flag.String("mirror_sts_from", "", "If set (e.g. 'google.com'), proxy the STS policy for this domain.")
	stsMode       = flag.String("sts_mode", "testing", "STS mode: 'testing' or 'enforce'.")
	stsMx         = flag.String("sts_mx", "", "Comma-separated 'mx' patterns.")
	stsMaxAge     = flag.String("sts_max_age", "2419200", "STS 'max_age'.")
)

func hostPolicy() autocert.HostPolicy {
	if *domains != "" {
		// If there is a whitelist, just use that.
		ds := strings.Split(*domains, ",")
		for i := range ds {
			ds[i] = "mta-sts." + ds[i]
		}
		return autocert.HostWhitelist(ds...)
	}
	return func(ctx context.Context, host string) error {
		// Check that the incoming host is a CNAME to us.
		if host != *myRealHost {
			if h, err := net.LookupCNAME(host); err != nil {
				return err
			} else if h != *myRealHost+"." {
				return fmt.Errorf("incoming host %s is not a cname for %s", host, *myRealHost)
			}
		}
		// And that we haven't tried this host too many times.
		hdir := filepath.Join(*certdir, hostsDir)
		p := filepath.Join(hdir, filepath.Clean(host))
		if s, err := os.Stat(p); err != nil {
			if !os.IsNotExist(err) {
				// Some other unexpected error here.
				return err
			}
		} else if s != nil && time.Now().Sub(s.ModTime()) < *tryCertNoMoreThan {
			// Too recently attempted this host.
			return fmt.Errorf("too recently attempted host %s", host)
		}
		// Touch the host file.
		if _, err := os.Stat(hdir); os.IsNotExist(err) {
			if err := os.MkdirAll(hdir, 0700); err != nil {
				return err
			}
		}
		_, err := os.Create(p)
		return err
	}
}

func main() {
	flag.Parse()
	if *domains == "" && *myRealHost == "" && !*serveHTTP {
		// Note that if we are serving HTTP, --domain and --my_real_host do nothing.
		fmt.Fprintln(os.Stderr, "Must specify --domain or --my_real_host for safety.")
		os.Exit(2)
	}
	if *mirrorStsFrom != "" && *stsMx != "" {
		fmt.Fprintln(os.Stderr, "Can only specify either --mirror_sts_from or --sts_mx options.")
		os.Exit(2)
	}
	if *mirrorStsFrom == "" && *stsMx == "" {
		fmt.Fprintln(os.Stderr, "Must specify either --mirror_sts_from or --sts_mx.")
		os.Exit(2)
	}
	if *stsMode != "" && *stsMode != "testing" && *stsMode != "none" && *stsMode != "enforce" {
		fmt.Fprintln(os.Stderr, "--sts_mode must be one of 'testing', 'enforce', 'none'.")
		os.Exit(2)
	}
	if *serveHTTP && (*staging || (*acmeEndpoint != "")) ||
	 (*staging && (*acmeEndpoint != "")) {
		fmt.Fprintln(os.Stderr, "Only one of --http, --staging, and --acme_endpoint can be used.")
		os.Exit(2)
	}

	// Serve policies.
	var stsMirror string
	var stsPolicy []byte
	if *mirrorStsFrom != "" {
		stsMirror = fmt.Sprintf("https://mta-sts.%v/.well-known/mta-sts.txt", *mirrorStsFrom)
	} else {
		mxs := strings.Split(*stsMx, ",")
		for i := range mxs {
			mxs[i] = "mx: " + mxs[i]
		}
		stsPolicy = []byte(fmt.Sprintf("version: STSv1\r\nmode: %s\r\nmax_age: %s\r\n%s\r\n", *stsMode, *stsMaxAge, strings.Join(mxs, "\r\n")))
	}
	http.HandleFunc("/.well-known/mta-sts.txt", func(w http.ResponseWriter, req *http.Request) {
		log.Printf("%s : %s : %s\n", req.RemoteAddr, req.Host, req.UserAgent())
		var policy *[]byte
		if stsMirror != "" {
			response, err := http.Get(stsMirror)
			if err != nil {
				log.Printf("Error fetching %s: %v\n", stsMirror, err)
			}
			defer response.Body.Close()
			contents, err := ioutil.ReadAll(response.Body)
			if err != nil {
				log.Printf("Error fetching %s: %v\n", stsMirror, err)
			}
			policy = &contents
		} else {
			policy = &stsPolicy
		}
		_, err := w.Write(*policy)
		if err != nil {
			log.Printf("Error serving: %v\n", err)
		}
	})

	srv := &http.Server{
		Handler: http.DefaultServeMux,
	}

	if *serveHTTP {
		srv.Addr = ":http"
	} else {
		// Initialize certificate manager.
		cm := &autocert.Manager{
			Cache:      autocert.DirCache(filepath.Join(*certdir, certsDir)),
			Prompt:     autocert.AcceptTOS,
			HostPolicy: hostPolicy(),
		}
		if *acmeEndpoint != "" {
			cm.Client = &acme.Client{DirectoryURL: *acmeEndpoint}
		} else if *staging {
			cm.Client = &acme.Client{DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory"}
		}
		srv.Addr = ":https"
		srv.TLSConfig = cm.TLSConfig()
		srv.TLSConfig.MinVersion = tls.VersionTLS12
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	port = ":" + port

	if *serveHTTP {
		// Serve the HTTP endpoint.
		srv.Addr = port
		fmt.Fprintln(os.Stderr, srv.ListenAndServe())
	} else {
		// Serve the HTTPS endpoint.
		fmt.Fprintln(os.Stderr, srv.ListenAndServeTLS("", ""))
		// Serve nothing on HTTP so that Docker hosts who want
		// to just know we are "live" can check.
		go func() {
			fmt.Fprintln(os.Stderr, http.ListenAndServe(port, http.NotFoundHandler()))
		}()

	}
	os.Exit(2)
}
