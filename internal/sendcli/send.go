// Package sendcli implements `landrop send`, which pushes a file or stdin to
// the LanDrop instance running on this machine via its local API.
package sendcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/lupguo/landrop/internal/i18n"
	"github.com/lupguo/landrop/internal/instance"
	"github.com/lupguo/landrop/internal/store"
)

// Run executes `landrop send`. args excludes the "send" word. All user-facing
// messages, including errors, are written to out; a non-nil error means the
// send failed and the caller should exit non-zero.
func Run(args []string, stdin io.Reader, out io.Writer, lang string) error {
	if len(args) != 1 || args[0] == "" {
		fmt.Fprintln(out, i18n.T(lang, i18n.KeySendUsage))
		return errors.New("bad arguments")
	}
	info, err := instance.Read(instance.DefaultPath())
	if err != nil {
		return notRunning(out, lang)
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", info.Port)

	var req *http.Request
	if args[0] == "-" {
		if stdin == nil {
			stdin = bytes.NewReader(nil)
		}
		buf, err := io.ReadAll(io.LimitReader(stdin, store.MaxText+1))
		if err != nil {
			return fail(out, lang, err)
		}
		if len(buf) <= store.MaxText {
			req, err = http.NewRequest("POST", base+"/api/local/text", bytes.NewReader(buf))
		} else {
			body := io.MultiReader(bytes.NewReader(buf), stdin)
			req, err = http.NewRequest("POST", base+"/api/local/upload?name=stdin.txt", body)
		}
		if err != nil {
			return err
		}
	} else {
		f, err := os.Open(args[0])
		if err != nil {
			return fail(out, lang, err)
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			return fail(out, lang, err)
		}
		if st.IsDir() {
			msg := i18n.T(lang, i18n.KeySendIsDir, args[0])
			fmt.Fprintln(out, msg)
			return errors.New(msg)
		}
		req, err = http.NewRequest("POST", base+"/api/local/upload?name="+url.QueryEscape(filepath.Base(args[0])), f)
		if err != nil {
			return err
		}
		req.ContentLength = st.Size()
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-LanDrop-Secret", info.Secret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var op *net.OpError
		if errors.As(err, &op) && op.Op == "dial" {
			return notRunning(out, lang)
		}
		return fail(out, lang, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var e struct {
			Error string `json:"error"`
		}
		json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&e)
		if e.Error == "" {
			e.Error = resp.Status
		}
		msg := i18n.T(lang, i18n.KeySendFailed, e.Error)
		fmt.Fprintln(out, msg)
		return errors.New(msg)
	}
	io.Copy(io.Discard, resp.Body)
	fmt.Fprintln(out, i18n.T(lang, i18n.KeySendOK))
	return nil
}

// fail reports a local error (I/O, network) through send.failed.
func fail(out io.Writer, lang string, err error) error {
	fmt.Fprintln(out, i18n.T(lang, i18n.KeySendFailed, err.Error()))
	return err
}

func notRunning(out io.Writer, lang string) error {
	msg := i18n.T(lang, i18n.KeySendNotRunning)
	fmt.Fprintln(out, msg)
	return errors.New(msg)
}
