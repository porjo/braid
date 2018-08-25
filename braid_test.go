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

package braid

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func TestFetchFile(t *testing.T) {
	var fileSize int64 = 1 << 20 * 5 // 5 MiB
	var jobs int = 2
	var filename string = "data.bin"
	var userAgent string = "braid test"

	b := &data{size: fileSize} // 5MiB data
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.UserAgent() != userAgent {
			t.Errorf("User-Agent header doesn't match: expecting '%s', got '%s'\n", userAgent, r.UserAgent())
			http.Error(w, "", 503)
			return
		}
		http.ServeContent(w, r, filename, time.Now(), b)
	}))
	defer ts.Close()

	var file *os.File

	ctx := context.Background()
	br, err := NewRequest()
	if err != nil {
		t.Fatal(err)
	}
	br.SetJobs(jobs)
	br.SetUserAgent(userAgent)

	logOut := ""
	logger := func(a string, b ...interface{}) {
		logOut += fmt.Sprintf(a, b)
	}

	SetLogger(logger)
	file, err = br.FetchFile(ctx, ts.URL, filename)
	if err != nil {
		t.Fatal(err)
	}

	var fstat os.FileInfo
	fstat, err = file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if fstat.Size() != fileSize {
		t.Fatalf("downloaded file size %d does not match server file size %d", fstat.Size(), fileSize)
	}

	file.Close()
	err = os.Remove(filename)
	if err != nil {
		t.Fatal(err)
	}

	// check braid stats match

	if br.Stats().TotalBytes != fileSize {
		t.Fatalf("stats TotalBytes doesn't match filesize: expected %d, got %d\n", fileSize, br.Stats().TotalBytes)
	}
	if br.Stats().ReadBytes != fileSize {
		t.Fatalf("stats ReadBytes doesn't match filesize: expected %d, got %d\n", fileSize, br.Stats().ReadBytes)
	}

	// check something was logged
	if logOut == "" {
		t.Fatalf("Braid.Logger log output was empty")
	}
}

func TestFetchFileFail(t *testing.T) {
	var filename string = "data.bin"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "", 500)
	}))
	defer ts.Close()

	var file *os.File

	ctx := context.Background()
	br, err := NewRequest()
	if err != nil {
		t.Fatal(err)
	}

	logOut := ""
	logger := func(a string, b ...interface{}) {
		logOut += fmt.Sprintf(a, b)
	}

	SetLogger(logger)
	file, err = br.FetchFile(ctx, ts.URL, filename)
	if err == nil {
		t.Fatalf("Expecting error from FetchFile but got nil")
	}
	file.Close()
	err = os.Remove(filename)
	if err != nil {
		t.Fatal(err)
	}
}

// data provides a way to generate a file of any size to be served by the test HTTP server
type data struct {
	sync.Mutex
	size  int64
	count int64
}

func (b *data) Read(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	i := len(p)
	if b.count+int64(i) > b.size {
		i = int(b.size - b.count)
	}
	if i == 0 {
		return 0, io.EOF
	}
	a := make([]byte, i)
	copy(a, p)
	b.count += int64(i)
	return i, nil
}

func (b *data) Seek(o int64, w int) (int64, error) {
	b.Lock()
	defer b.Unlock()
	if w == io.SeekEnd {
		b.count = b.size - o
	}
	if w == io.SeekCurrent {
		b.count += o
	}
	if w == io.SeekStart {
		b.count = o
	}

	if b.count < 0 {
		b.count = 0
		return 0, fmt.Errorf("bad count")
	}

	return b.count, nil
}
