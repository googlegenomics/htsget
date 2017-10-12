# htsget on GCS

This repository contains an implementation of the [htsget
protocol](http://samtools.github.io/hts-specs/htsget.html) that provides access
to reads data stored in Google Cloud Storage buckets.

# Building the server

In order to build the server, you will need the [Go](https://golang.org/) tool
chain (at least version 1.8).

Once Go is installed, you can build the server by running:

```
$ go get https://github.com/googlegenomics/htsget/htsget-server
```

This will produce a binary in $GOPATH/bin called htsget-server.

# Running the server

You can use htsget-server in one of two modes:

* Insecure mode for public resources.  When used in this way, the htsget-server
does not use TLS and does not authenticate requests it makes to Google Cloud
storage.  This is useful if you want to access public data via the htsget
protocol on a machine that is already running htslib based tools (like
samtools).

* Secure mode with authentication.  In this mode, the server requires a TLS
certificate and key to be passed as command line flags.  It will then listen on
all interfaces and accept requests secured via TLS.  Each request must contain
an OAuth2 Bearer Access Token which will be used to fetch data from GCS.

# Example usage

## Insecure mode

```
$ bin/htsget-server --port=1234 &
$ samtools flagstat http://localhost:1234/reads/public-bucket/test.bam
```

This will use htsget to retrieve data from 'test.bam' stored in the GCS bucket
'public-bucket'.

## Secure mode

```
$ bin/htsget-server --secure=true --port=443 --https_cert=server.crt --https_key=server.key &
$ export CURL_CA_BUNDLE=server.crt
$ export HTS_AUTH_LOCATION=/path/to/my-oauth2-token
$ samtools flagstat http://localhost:1234/reads/private-bucket/test.bam
```

The file `server.crt` and `server.key` can be generated using the
[generate_cert](https://golang.org/src/crypto/tls/generate_cert.go) tool that
comes with Go, or using `openssl`.

Note that you will require versions of `samtools` and `htslib` that support the
environment variables used above (`CURL_CA_BUNDLE` and `HTS_AUTH_LOCATION`).
This support was added in October of 2017.
