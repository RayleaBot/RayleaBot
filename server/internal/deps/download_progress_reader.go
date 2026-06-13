package deps

import (
	"errors"
	"io"
)

type progressReader struct {
	reader     io.Reader
	total      int64
	read       int64
	lastNotify int
	lastBytes  int64
	notify     func(downloadProgress)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.read += int64(n)
		r.emit(false)
	}
	if errors.Is(err, io.EOF) {
		r.emit(true)
	}
	return n, err
}

func (r *progressReader) emit(force bool) {
	if r.notify == nil {
		return
	}
	percent := prepareProgressPercent(r.read, r.total)
	if !force && r.total <= 0 && r.read-r.lastBytes < 1024*1024 {
		return
	}
	if !force && r.total > 0 && percent == r.lastNotify {
		return
	}
	r.lastNotify = percent
	r.lastBytes = r.read
	r.notify(downloadProgress{
		DownloadedBytes: r.read,
		TotalBytes:      r.total,
		Progress:        percent,
	})
}
