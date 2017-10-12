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

* localhost only "proxy" mode.  When used in this way, the htsget-server uses
the application default credentials stored on the machine it is running on to
access resources in GCS.  It listens only on localhost and does not use TLS.
This is useful if you want to access data via the htsget protocol on a machine
that is already running htslib based tools (like samtools).

* TLS serving mode.  In this mode, the server requires a TLS certificate and
key to be passed as command line flags.  It will then listen on all
interfaces and accept requests secured via TLS.  If the request contains an
OAuth2 Bearer Access Token, it will be used to fetch data from GCS.
Otherwise, the application default credentials are used as in the previous
mode.  This is useful when sharing data to other users or organizations via
the htsget protocol.

# Example usage

## localhost only mode

```
$ bin/htsget-server --port=1234 &
$ samtools flagstat http://localhost:1234/reads/my-bucket-name/test.bam
```

This will use htsget to retrieve data from 'test.bam' stored in the GCS bucket
'my-bucket-name'.

## TLS serving mode

```
$ bin/htsget-server --port=443 --https_cert=server.crt --https_key=server.key &
```

The file `server.crt` and `server.key` can be generated using the
[generate_cert](https://golang.org/src/crypto/tls/generate_cert.go) tool that
comes with Go, or using `openssl`.
