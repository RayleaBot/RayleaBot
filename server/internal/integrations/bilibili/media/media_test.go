package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestFetchReadsAllowedHdslbImage(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://i0.hdslb.com/bfs/face/up.jpg" {
			t.Fatalf("request url = %q, want hdslb image url", request.URL.String())
		}
		if request.Header.Get("Referer") != "https://www.bilibili.com/" {
			t.Fatalf("referer = %q, want Bilibili referer", request.Header.Get("Referer"))
		}
		if request.Header.Get("User-Agent") == "" {
			t.Fatal("expected user agent header")
		}
		if !strings.Contains(request.Header.Get("Accept"), "image/webp") {
			t.Fatalf("accept = %q, want image accept header", request.Header.Get("Accept"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"IMAGE/JPEG; charset=binary"}},
			Body:       io.NopCloser(strings.NewReader("jpeg-bytes")),
			Request:    request,
		}, nil
	})}

	resource, err := Fetch(context.Background(), client, " https://i0.hdslb.com/bfs/face/up.jpg ")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if resource.ContentType != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", resource.ContentType)
	}
	if string(resource.Body) != "jpeg-bytes" {
		t.Fatalf("body = %q, want jpeg-bytes", string(resource.Body))
	}
}

func TestFetchReadsAllowedWeiboAvatar(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://tvax1.sinaimg.cn/crop.0.0.512.512.180/fixture.jpg?KID=imgbed,tva" {
			t.Fatalf("request url = %q, want Weibo avatar url", request.URL.String())
		}
		if request.Header.Get("Referer") != "https://weibo.com/" {
			t.Fatalf("referer = %q, want Weibo referer", request.Header.Get("Referer"))
		}
		if request.Header.Get("User-Agent") == "" {
			t.Fatal("expected user agent header")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/webp"}},
			Body:       io.NopCloser(strings.NewReader("webp-bytes")),
			Request:    request,
		}, nil
	})}

	resource, err := Fetch(context.Background(), client, " https://tvax1.sinaimg.cn/crop.0.0.512.512.180/fixture.jpg?KID=imgbed,tva ")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if resource.ContentType != "image/webp" {
		t.Fatalf("content type = %q, want image/webp", resource.ContentType)
	}
	if string(resource.Body) != "webp-bytes" {
		t.Fatalf("body = %q, want webp-bytes", string(resource.Body))
	}
}

func TestNormalizeURLAllowsSupportedImageHosts(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "subdomain bfs",
			value: "https://i0.hdslb.com/bfs/face/up.jpg",
			want:  "https://i0.hdslb.com/bfs/face/up.jpg",
		},
		{
			name:  "root fs",
			value: "https://hdslb.com/fs/archive/up.webp",
			want:  "https://hdslb.com/fs/archive/up.webp",
		},
		{
			name:  "weibo avatar",
			value: "https://tvax1.sinaimg.cn/crop.0.0.512.512.180/fixture.jpg?KID=imgbed,tva",
			want:  "https://tvax1.sinaimg.cn/crop.0.0.512.512.180/fixture.jpg?KID=imgbed,tva",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeURL(tc.value)
			if err != nil {
				t.Fatalf("normalizeURL returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("normalizeURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFetchRejectsUnsupportedURL(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected upstream request: %s", request.URL.String())
		return nil, nil
	})}
	for _, value := range []string{
		"",
		"http://i0.hdslb.com/bfs/face/up.jpg",
		"https://example.com/bfs/face/up.jpg",
		"https://evilhdslb.com/bfs/face/up.jpg",
		"https://i0.hdslb.com/not-bfs/up.jpg",
		"https://i0.hdslb.com/bfs/face/up.jpg?x=1",
		"https://user@i0.hdslb.com/bfs/face/up.jpg",
		"https://evilsinaimg.cn/crop.0.0.512.512.180/fixture.jpg",
		"https://tvax1.sinaimg.cn/",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			_, err := Fetch(context.Background(), client, value)
			if !errors.Is(err, ErrUnsupportedURL) {
				t.Fatalf("Fetch error = %v, want ErrUnsupportedURL", err)
			}
		})
	}
}

func TestFetchRejectsUpstreamFailure(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(strings.NewReader("forbidden")),
			Request:    request,
		}, nil
	})}

	_, err := Fetch(context.Background(), client, "https://i0.hdslb.com/bfs/face/up.jpg")
	if !errors.Is(err, ErrReadFailed) {
		t.Fatalf("Fetch error = %v, want ErrReadFailed", err)
	}
}

func TestFetchRejectsUnsupportedContentType(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("not an image")),
			Request:    request,
		}, nil
	})}

	_, err := Fetch(context.Background(), client, "https://i0.hdslb.com/bfs/face/up.jpg")
	if !errors.Is(err, ErrUnsupportedContentType) {
		t.Fatalf("Fetch error = %v, want ErrUnsupportedContentType", err)
	}
}

func TestFetchRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", maxMediaBytes+1))),
			Request:    request,
		}, nil
	})}

	_, err := Fetch(context.Background(), client, "https://i0.hdslb.com/bfs/face/up.png")
	if !errors.Is(err, ErrReadFailed) {
		t.Fatalf("Fetch error = %v, want ErrReadFailed", err)
	}
}
