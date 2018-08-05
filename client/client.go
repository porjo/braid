package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/porjo/braid"
)

func main() {
	var url, filename string
	var jobs int
	var err error

	flag.StringVar(&url, "url", "", "URL to fetch")
	flag.IntVar(&jobs, "jobs", 5, "number of jobs")
	flag.StringVar(&filename, "filename", "", "filename to write result to")
	flag.Parse()

	if url == "" || filename == "" {
		fmt.Println("url and filename must be specified")
		flag.PrintDefaults()
		os.Exit(1)
	}

	var b *braid.Request
	ctx := context.Background()
	b, err = braid.NewRequest(ctx, filename)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	b.SetJobs(2)
	braid.SetLogger(log.Printf)
	_, err = b.Fetch(url)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
