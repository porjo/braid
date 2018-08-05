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
	jobs int
	file *os.File
	ctx  context.Context
	wg   sync.WaitGroup
	mu   sync.Mutex
}

// NewRequest returns a new request.
// Filename must be writable, will be created if missing and will be truncated.
func NewRequest(ctx context.Context, filename string) (*Request, error) {
	var err error

	r := &Request{
		ctx:  ctx,
		jobs: DefaultJobs,
	}

	r.file, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// SetJobs sets the number of parallel requests that will be made. DefaultJobs is used by default.
func (r *Request) SetJobs(jobs int) {
	r.jobs = jobs
}

// Fetch fetches the resource, returning the result as an *os.File.
func (r *Request) Fetch(url string) (*os.File, error) {
	var err error
	var length int
	var res *http.Response

	res, err = http.Head(url)
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
	logger("content length %d, chunksize %d, last %d\n", length, chunkSize, chunkSizeLast)

	r.wg.Add(r.jobs)

	for i := 0; i < r.jobs; i++ {

		min := chunkSize * i
		max := chunkSize * (i + 1)

		if i == r.jobs-1 {
			max += chunkSizeLast
		}

		go fetch(r.ctx, min, max, url, r.file, &r.wg, &r.mu)

	}
	r.wg.Wait()

	return r.file, nil
}

func fetch(ctx context.Context, min int, max int, url string, file *os.File, wg *sync.WaitGroup, mu *sync.Mutex) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger("a: %s\n", err)
		return
	}
	req = req.WithContext(ctx)
	range_header := "bytes=" + strconv.Itoa(min) + "-" + strconv.Itoa(max-1)
	req.Header.Add("Range", range_header)

	logger("fetch range %d-%d\n", min, max)

	resp, err := client.Do(req)
	if err != nil {
		logger("a: %s\n", err)
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
				logger("b: %s\n", err)
				return
			}
		}
		var count int
		mu.Lock()
		count, err = file.WriteAt(line, int64(min+read))
		mu.Unlock()
		read += len(line)
		if err != nil {
			logger("c: %s\n", err)
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
	wg.Done()
}
