package main

import (
	"golang.org/x/crypto/acme/autocert"

	"context"
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
		if h, err := net.LookupCNAME(host); err != nil {
			return err
		} else if h != *myRealHost {
			return fmt.Errorf("incoming host %s is not a cname for %s", host, myRealHost)
		}
		// And that we haven't tried this host too many times.
		p := filepath.Join(*certdir, hostsDir, filepath.Clean(host))
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
		_, err := os.Create(p)
		return err
	}
}

func main() {
	flag.Parse()
	if *domains == "" && *myRealHost == "" {
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
		stsPolicy = []byte(fmt.Sprintf("version: STSv1\nmode: %s\nmax_age: %s\n%s", *stsMode, *stsMaxAge, strings.Join(mxs, "\n")))
	}
	http.HandleFunc("/.well-known/mta-sts.txt", func(w http.ResponseWriter, req *http.Request) {
		log.Println("%s : %s : %s\n", req.RemoteAddr, req.Host, req.UserAgent())
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

	// Initialize certificate manager.
	cm := &autocert.Manager{
		Cache:      autocert.DirCache(filepath.Join(*certdir, certsDir)),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: hostPolicy(),
	}

	srv := &http.Server{
		Addr:      ":https",
		TLSConfig: cm.TLSConfig(),
		Handler:   http.DefaultServeMux,
	}

	fmt.Fprintln(os.Stderr, srv.ListenAndServeTLS("", ""))
	os.Exit(2)
}
