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

// Package braid provides a way to GET a single HTTP resource using multiple parallel requests.
package braid

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
)

type Logger func(string, ...interface{})

var logger Logger = func(a string, b ...interface{}) {}

// SetLogger sets where log should be sent
// by default log is muted
func SetLogger(l Logger) {
	// wrap supplied logger & prepend the library name
	logger = func(a string, b ...interface{}) {
		l("braid: "+a, b...)
	}
}

// DefaultJobs is the number of parallel HTTP requests to be made by default.
const DefaultJobs = 5

type Request struct {
	jobs      int
	url       string
	wg        sync.WaitGroup
	mu        sync.Mutex
	userAgent string

	// these are covered by mutex
	file  *os.File
	stats []Stat
}

type Stat struct {
	TotalBytes int64
	ReadBytes  int64
}

// NewRequest returns a new request.
func NewRequest() (*Request, error) {
	r := &Request{
		jobs: DefaultJobs,
	}

	return r, nil
}

// SetJobs sets the number of parallel requests that will be made. DefaultJobs is used by default.
func (r *Request) SetJobs(jobs int) {
	r.jobs = jobs
}

// SetUserAgent sets the 'User-Agent' HTTP header used when making requests
func (r *Request) SetUserAgent(userAgent string) {
	r.userAgent = userAgent
}

// Stats retrieves current statistics. It is thread safe and can be called from a goroutine.
func (r *Request) Stats() Stat {
	stat := Stat{}
	r.mu.Lock()
	for _, s := range r.stats {
		stat.TotalBytes += s.TotalBytes
		stat.ReadBytes += s.ReadBytes
	}
	r.mu.Unlock()

	return stat
}

// FetchFile fetches the resource, returning the result as an *os.File
// The caller is responsible for closing the returned file.
// Filename must be writable, will be created if missing and will be truncated.
func (r *Request) FetchFile(ctx context.Context, url, filename string) (*os.File, error) {
	var err error
	var length int
	var req *http.Request
	var res *http.Response

	r.file, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return nil, err
	}

	r.url = url
	client := &http.Client{}
	req, err = http.NewRequest("HEAD", r.url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if r.userAgent != "" {
		req.Header.Set("User-Agent", r.userAgent)
	}
	res, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching HEAD: %s\n", err)
	}

	headers := res.Header
	length, err = strconv.Atoi(headers["Content-Length"][0])
	if err != nil {
		return nil, err
	}

	if r.jobs <= 0 {
		r.jobs = 1
	}
	chunkSize := length / r.jobs
	chunkSizeLast := length % r.jobs

	r.stats = make([]Stat, r.jobs)
	r.wg.Add(r.jobs)

	logger("fetching %s\n", r.url)
	logger("launching %d jobs\n", r.jobs)

	errChan := make(chan error)
	for i := 0; i < r.jobs; i++ {

		min := chunkSize * i
		max := chunkSize * (i + 1)

		if i == r.jobs-1 {
			max += chunkSizeLast
		}

		r.stats[i].TotalBytes = int64(max - min)
		go r.fetchFile(ctx, min, max, i, errChan)

	}

	quitChan := make(chan struct{})
	errors := ""
	go func() {
		for {
			select {
			case err := <-errChan:
				errors += "\n" + err.Error()
			case <-quitChan:
				return
			}
		}
	}()

	r.wg.Wait()
	close(quitChan)

	if errors != "" {
		return r.file, fmt.Errorf("%s", errors)
	} else {
		return r.file, nil
	}
}

func (r *Request) fetchFile(ctx context.Context, min int, max int, jobID int, errChan chan error) {
	defer r.wg.Done()
	client := &http.Client{}
	req, err := http.NewRequest("GET", r.url, nil)
	if err != nil {
		errChan <- err
		return
	}
	req = req.WithContext(ctx)
	range_header := "bytes=" + strconv.Itoa(min) + "-" + strconv.Itoa(max-1)
	req.Header.Add("Range", range_header)

	if r.userAgent != "" {
		req.Header.Set("User-Agent", r.userAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		errChan <- err
		return
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	read := 0
	for {
		var end bool
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				end = true
			} else {
				return
			}
		}
		var count int
		r.mu.Lock()
		count, err = r.file.WriteAt(line, int64(min+read))
		read += len(line)
		r.stats[jobID].ReadBytes = int64(read)
		r.mu.Unlock()
		if err != nil {
			errChan <- err
			return
		}

		if count != len(line) {
			logger("write error: expected %d bytes, got %d bytes\n", len(line), count)
			return
		}

		if end {
			break
		}
	}
}
