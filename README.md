# Braid

[![Documentation](https://godoc.org/github.com/porjo/braid?status.svg)](http://godoc.org/github.com/porjo/braid)
[![Build Status](https://travis-ci.org/porjo/braid.svg?branch=master)](https://travis-ci.org/porjo/braid)

A Go library for fetching a HTTP resource using parallel GET requests

Example:

```Go
	var b *braid.Request
	var f *os.File

	b, err = braid.NewRequest()
	if err != nil {
		os.Exit(err)
	}
	b.SetJobs(3) // set number of parallel requests. Defaults to 5
	ctx := context.Background()
	f, err = b.FetchFile(ctx, url, filename)
	if err != nil {
		os.Exit(err)
	}
	// f.Close()
```

See `cmd/braid/braid.go` for a working example.

