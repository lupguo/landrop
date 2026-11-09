package server

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lupguo/landrop/internal/store"
)

func multipartBody(t *testing.T, filename, ctype string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(filename)+`"`)
	if ctype != "" {
		h.Set("Content-Type", ctype)
	}
	pw, err := mw.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	pw.Write(data)
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func doUpload(t *testing.T, c *http.Client, base, filename, ctype string, data []byte) (*http.Response, []byte) {
	t.Helper()
	body, ct := multipartBody(t, filename, ctype, data)
	req, _ := http.NewRequest("POST", base+"/api/upload", body)
	req.Header.Set("Origin", base)
	req.Header.Set("Content-Type", ct)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func storeFiles(t *testing.T, dir string) []string {
	t.Helper()
	es, _ := os.ReadDir(dir)
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	return names
}

func TestUploadDownloadRoundTrip(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	sse := openSSE(t, c, ts, "x", "")
	data := make([]byte, 3<<20)
	rand.Read(data)
	resp, b := doUpload(t, c, ts.URL, "app.log", "", data)
	if resp.StatusCode != 201 {
		t.Fatalf("status %d %s", resp.StatusCode, b)
	}
	var it store.Item
	decode(t, b, &it)
	ev := sse.next(t, 3*time.Second, isType("item.created"))
	var evItem store.Item
	decode(t, []byte(ev.Data), &evItem)
	if evItem.File == nil || evItem.File.Size != int64(len(data)) {
		t.Fatalf("sse item %+v", evItem)
	}
	r2, got := do(t, c, ts, "GET", "/api/items/"+it.ID+"/raw", "")
	if r2.StatusCode != 200 {
		t.Fatalf("raw status %d", r2.StatusCode)
	}
	if sha256.Sum256(got) != sha256.Sum256(data) {
		t.Fatal("hash mismatch")
	}
	if cd := r2.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, "filename*=UTF-8''app.log") {
		t.Fatalf("disposition %q", cd)
	}
}

func TestUploadOddFilenames(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`../../x`, "x"}, {`a/b\c.txt`, "c.txt"}, {`日志 "v2".log`, `日志 "v2".log`}, {``, "file"},
		{`a'b*(1);50%.txt`, `a'b*(1);50%.txt`},
	} {
		ts, d := newTestServer(t, nil)
		c := login(t, ts)
		resp, b := doUpload(t, c, ts.URL, tc.in, "", []byte("hello"))
		if resp.StatusCode != 201 {
			t.Fatalf("%q: status %d %s", tc.in, resp.StatusCode, b)
		}
		var it store.Item
		decode(t, b, &it)
		if it.File.Name != tc.want {
			t.Fatalf("%q: name %q want %q", tc.in, it.File.Name, tc.want)
		}
		for _, n := range storeFiles(t, d.Store.Dir()) {
			if n != it.ID {
				t.Fatalf("%q: stray file %q", tc.in, n)
			}
		}
		r2, _ := do(t, c, ts, "GET", "/api/items/"+it.ID+"/raw", "")
		cd := r2.Header.Get("Content-Disposition")
		_, enc, ok := strings.Cut(cd, "filename*=UTF-8''")
		if !ok {
			t.Fatalf("disposition %q", cd)
		}
		dec, err := url.PathUnescape(enc)
		if err != nil || dec != it.File.Name {
			t.Fatalf("decoded %q err %v want %q", dec, err, it.File.Name)
		}
		if strings.Contains(cd, "日") {
			t.Fatalf("non-ascii in fallback: %q", cd)
		}
	}
}

func TestUploadTooLarge(t *testing.T) {
	ts, d := newTestServer(t, func(d *Deps) { d.MaxSize = 1024 })
	c := login(t, ts)
	resp, b := doUpload(t, c, ts.URL, "big.bin", "", make([]byte, 2048))
	wantErr(t, resp, b, 413, "too_large")
	if n := len(d.Store.List(0)); n != 0 {
		t.Fatalf("%d items", n)
	}
	if f := storeFiles(t, d.Store.Dir()); len(f) != 0 {
		t.Fatalf("leftover %v", f)
	}
}

func TestUploadDiskFull(t *testing.T) {
	old := freeSpace
	freeSpace = func(string) (uint64, error) { return 1 << 20, nil }
	defer func() { freeSpace = old }()
	ts, d := newTestServer(t, nil)
	c := login(t, ts)
	resp, b := doUpload(t, c, ts.URL, "a.bin", "", make([]byte, 1000))
	wantErr(t, resp, b, 507, "disk_full")
	if len(d.Store.List(0)) != 0 || len(storeFiles(t, d.Store.Dir())) != 0 {
		t.Fatal("leftovers")
	}
}

func TestUploadNoFile(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("other", "x")
	mw.Close()
	req, _ := http.NewRequest("POST", ts.URL+"/api/upload", &buf)
	req.Header.Set("Origin", ts.URL)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	wantErr(t, resp, b, 400, "no_file")
}

func TestUploadOnFileStored(t *testing.T) {
	got := make(chan store.Item, 1)
	ts, _ := newTestServer(t, func(d *Deps) { d.OnFileStored = func(it store.Item) { got <- it } })
	c := login(t, ts)
	doUpload(t, c, ts.URL, "a.txt", "", []byte("x"))
	// Called synchronously, before the response is written.
	select {
	case it := <-got:
		if it.File.Name != "a.txt" {
			t.Fatal(it)
		}
	default:
		t.Fatal("OnFileStored not called before the response")
	}
}

func TestUploadAbortLeavesNothing(t *testing.T) {
	ts, d := newTestServer(t, nil)
	c := login(t, ts)
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	req, _ := http.NewRequest("POST", ts.URL+"/api/upload", pr)
	req.Header.Set("Origin", ts.URL)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if resp, err := c.Do(req); err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()
	pwr, _ := mw.CreateFormFile("file", "x.bin")
	pwr.Write(make([]byte, 100<<10))
	time.Sleep(100 * time.Millisecond)
	pw.CloseWithError(io.ErrUnexpectedEOF)
	<-done
	deadline := time.Now().Add(time.Second)
	for {
		files := storeFiles(t, d.Store.Dir())
		if len(files) == 0 && len(d.Store.List(0)) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("leftovers: %v, items %d", files, len(d.Store.List(0)))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestInlineOnlyForSafeImages(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	up := func(name, mt string) string {
		resp, b := doUpload(t, c, ts.URL, name, mt, []byte("data"))
		if resp.StatusCode != 201 {
			t.Fatalf("%d %s", resp.StatusCode, b)
		}
		var it store.Item
		decode(t, b, &it)
		return it.ID
	}
	png, svg := up("a.png", "image/png"), up("a.svg", "image/svg+xml")
	for _, mt := range []string{"image/png; charset=x", "IMAGE/PNG"} {
		id := up("b.png", mt)
		r, _ := do(t, c, ts, "GET", "/api/items/"+id+"/raw?inline=1", "")
		if !strings.HasPrefix(r.Header.Get("Content-Disposition"), "inline") {
			t.Fatalf("%s: %v", mt, r.Header)
		}
	}
	html := up("a.html", "text/html")
	if r, _ := do(t, c, ts, "GET", "/api/items/"+html+"/raw?inline=1", ""); !strings.HasPrefix(r.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("html: %v", r.Header)
	}
	r, _ := do(t, c, ts, "GET", "/api/items/"+png+"/raw?inline=1", "")
	if !strings.HasPrefix(r.Header.Get("Content-Disposition"), "inline") || r.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("png: %v", r.Header)
	}
	if r.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff missing")
	}
	r, _ = do(t, c, ts, "GET", "/api/items/"+svg+"/raw?inline=1", "")
	if !strings.HasPrefix(r.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("svg: %v", r.Header)
	}
	r, _ = do(t, c, ts, "GET", "/api/items/"+png+"/raw", "")
	if !strings.HasPrefix(r.Header.Get("Content-Disposition"), "attachment") {
		t.Fatal("png without inline must be attachment")
	}
}

func TestRawTextAndMissing(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	_, b := do(t, c, ts, "POST", "/api/text", `{"content":"hi there"}`)
	var it store.Item
	decode(t, b, &it)
	r, body := do(t, c, ts, "GET", "/api/items/"+it.ID+"/raw", "")
	if string(body) != "hi there" || r.Header.Get("Content-Type") != "text/plain; charset=utf-8" ||
		!strings.Contains(r.Header.Get("Content-Disposition"), it.ID+".txt") {
		t.Fatalf("%v %q", r.Header, body)
	}
	r, _ = do(t, c, ts, "GET", "/api/items/nope/raw", "")
	if r.StatusCode != 404 {
		t.Fatal(r.StatusCode)
	}
	r, _ = do(t, http.DefaultClient, ts, "GET", "/api/items/"+it.ID+"/raw", "")
	if r.StatusCode != 401 {
		t.Fatalf("unauth: %d", r.StatusCode)
	}
}

func TestContentDisposition(t *testing.T) {
	got := contentDisposition("attachment", "日\"\\\n.txt")
	want := `attachment; filename="____.txt"; filename*=UTF-8''%E6%97%A5%22%5C%0A.txt`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestRangeDownload(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	_, b := doUpload(t, c, ts.URL, "r.bin", "", []byte("0123456789abcdef"))
	var it store.Item
	decode(t, b, &it)
	r, body := do(t, c, ts, "GET", "/api/items/"+it.ID+"/raw", "", "Range", "bytes=0-9")
	if r.StatusCode != 206 || string(body) != "0123456789" {
		t.Fatalf("%d %q", r.StatusCode, body)
	}
}

// genReader yields n bytes without allocating.
type genReader struct{ n int64 }

func (g *genReader) Read(p []byte) (int, error) {
	if g.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > g.n {
		p = p[:g.n]
	}
	for i := range p {
		p[i] = byte(i)
	}
	g.n -= int64(len(p))
	return len(p), nil
}

func TestStreamingMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("large streaming test")
	}
	const size = 512 << 20
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	c.Timeout = 0

	heap := func() uint64 {
		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return ms.HeapInuse
	}
	base := heap()
	var peak uint64
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		for {
			select {
			case <-stop:
				return
			case <-time.After(250 * time.Millisecond):
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				if ms.HeapInuse > peak {
					peak = ms.HeapInuse
				}
			}
		}
	}()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		w, _ := mw.CreateFormFile("file", "big.bin")
		_, err := io.Copy(w, &genReader{n: size})
		mw.Close()
		pw.CloseWithError(err)
	}()
	req, _ := http.NewRequest("POST", ts.URL+"/api/upload", pr)
	req.Header.Set("Origin", ts.URL)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d %s", resp.StatusCode, b)
	}
	var it store.Item
	json.Unmarshal(b, &it)
	if it.File.Size != size {
		t.Fatalf("size %d", it.File.Size)
	}

	gr, err := c.Get(ts.URL + "/api/items/" + it.ID + "/raw")
	if err != nil {
		t.Fatal(err)
	}
	n, _ := io.Copy(io.Discard, gr.Body)
	gr.Body.Close()
	if n != size {
		t.Fatalf("downloaded %d", n)
	}
	close(stop)
	<-sampled
	after := heap()
	delta := int64(after) - int64(base)
	t.Logf("HeapInuse base=%dMiB after=%dMiB delta=%dMiB peak-sampled=%dMiB", base>>20, after>>20, delta>>20, peak>>20)
	if delta > 64<<20 {
		t.Fatalf("heap grew %d MiB", delta>>20)
	}
	if peak > base && int64(peak-base) > 64<<20 {
		t.Fatalf("peak heap grew %d MiB", (peak-base)>>20)
	}
}
