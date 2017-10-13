# htsget on GCS

This repository contains an implementation of the [htsget
protocol](http://samtools.github.io/hts-specs/htsget.html) that provides access
to reads data stored in Google Cloud Storage buckets.

Currently, only BAM is supported and the BAM index file must be colocated with
the BAM file (that is, `sample.bam` and `sample.bam.bai` must be in the same
GCS bucket).

CRAM support will be added in the very near future.

# Building the server

In order to build the server, you will need the [Go](https://golang.org/) tool
chain (at least version 1.8).

Once Go is installed, you can build the server by running:

```
$ go get https://github.com/googlegenomics/htsget/htsget-server
```

This will produce a binary in $GOPATH/bin called htsget-server.

# Usage

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

## Required file layout

In either mode, read requests identify the bucket and object (file) to read.
As an example, `/reads/testing/123.bam` will cause the server to try to access
the GCS bucket 'testing' and read two objects: `123.bam` and `123.bam.bai`.
The index file MUST be in the same bucket and have the `.bai` suffix.

# Running the server

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

# Known Issues

* The server isn't very efficient at limiting what reads are returned.  This is
an area we are actively working to improve (see issue #26).

* Filters on fields are ignored.  The server does not implement any filtering
beyond read range and reference name filters.  We do not currently plan to add
support for this.  If this is important to you, please file an issue and let us
know.
