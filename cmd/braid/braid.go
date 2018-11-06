// +build !test

/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/porjo/braid"
)

type chanChan chan chan struct{}

func main() {
	var url, filename string
	var jobs int
	var err error
	var file *os.File

	flag.StringVar(&url, "url", "", "URL to fetch")
	flag.IntVar(&jobs, "jobs", 5, "number of jobs")
	flag.StringVar(&filename, "filename", "", "filename to write result to")
	flag.Parse()

	if url == "" || filename == "" {
		fmt.Println("url and filename must be specified")
		flag.PrintDefaults()
		os.Exit(1)
	}

	var r *braid.Request
	ctx := context.Background()
	r, err = braid.NewRequest()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	r.SetJobs(jobs)
	braid.SetLogger(log.Printf)
	var quitChan chanChan
	quitChan = make(chanChan)
	go Progress(quitChan, r)
	file, err = r.FetchFile(ctx, url, filename)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	file.Close()
	quit := make(chan struct{})
	quitChan <- quit
	<-quit
}

func Progress(quitChan chanChan, r *braid.Request) {
	ticker := time.Tick(time.Second)
	for {
		select {
		case <-ticker:
			stats := r.Stats()
			fmt.Printf("%+v\n", stats)
		case ch := <-quitChan:
			// final newline
			fmt.Println()
			close(ch)
			return
		}
	}
}
