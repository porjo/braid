package braid

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	var fileSize int64 = 1 << 20 * 5 // 5 MiB
	var jobs int = 2
	var filename string = "data.bin"

	b := &data{size: fileSize} // 5MiB data
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, filename, time.Now(), b)
	}))
	defer ts.Close()

	var file *os.File

	ctx := context.Background()
	br, err := NewRequest(ctx, filename)
	if err != nil {
		t.Error(err)
		return
	}
	br.SetJobs(jobs)
	file, err = br.Fetch(ts.URL)
	if err != nil {
		t.Error(err)
		return
	}

	var stat os.FileInfo
	stat, err = file.Stat()
	if err != nil {
		t.Error(err)
		return
	}

	if stat.Size() != fileSize {
		t.Errorf("downloaded file size %d does not match server file size %d", stat.Size(), fileSize)
		return
	}

	file.Close()
	err = os.Remove("blob.dat")
	if err != nil {
		t.Error(err)
		return
	}
}

// data provides a way to generate a file of any size to be served by the test HTTP server
type data struct {
	size  int64
	count int64
}

func (b *data) Read(p []byte) (int, error) {
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
