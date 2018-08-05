# Braid

[![Documentation](https://godoc.org/github.com/porjo/braid?status.svg)](http://godoc.org/github.com/porjo/braid)
[![Build Status](https://travis-ci.org/porjo/braid.svg?branch=master)](https://travis-ci.org/porjo/braid)

A Go library for fetching a HTTP resource using parallel GET requests

Example:

```Go
	var b *braid.Request
	var f *os.File

	ctx := context.Background()
	b, _ = braid.NewRequest(ctx, filename)
	f, _ = b.Fetch(url)
```

See `client/client.go` for a working example.

