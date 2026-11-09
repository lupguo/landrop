// Package app wires LanDrop's packages into the running server.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/lupguo/landrop/internal/auth"
	"github.com/lupguo/landrop/internal/autosave"
	"github.com/lupguo/landrop/internal/config"
	"github.com/lupguo/landrop/internal/discovery"
	"github.com/lupguo/landrop/internal/hub"
	"github.com/lupguo/landrop/internal/i18n"
	"github.com/lupguo/landrop/internal/instance"
	"github.com/lupguo/landrop/internal/server"
	"github.com/lupguo/landrop/internal/share"
	"github.com/lupguo/landrop/internal/store"
	"github.com/lupguo/landrop/internal/termui"
	"github.com/lupguo/landrop/internal/version"
	"github.com/lupguo/landrop/internal/webui"
)

// Test hooks.
var (
	sweepInterval = time.Minute
	listenTries   = 10
	announce      = discovery.Announce
	printBanner   = termui.Print
)

const shutdownTimeout = 5 * time.Second

// lockedWriter serializes writes from the app's goroutines.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// Run serves LanDrop until ctx is done, then shuts down and cleans up.
func Run(ctx context.Context, cfg config.Config, out io.Writer) error {
	out = &lockedWriter{w: out}

	// 1. listener
	ln, port, err := Listen(cfg.Port, listenTries)
	if err != nil {
		// Keep only the OS error (e.g. "bind: permission denied") as detail.
		if inner := errors.Unwrap(err); inner != nil {
			err = inner
		}
		return fmt.Errorf("%s (%w)", i18n.T(cfg.Lang, i18n.KeyErrPortExhausted, cfg.Port, cfg.Port+listenTries-1), err)
	}
	defer ln.Close()

	// 2. core state
	// An unpredictable name: a fixed landrop-<pid> could be pre-created
	// (e.g. as a symlink) by another local user.
	storeDir, err := os.MkdirTemp("", "landrop-")
	if err != nil {
		return err
	}
	st, err := store.New(storeDir, cfg.TTL, time.Now)
	if err != nil {
		os.RemoveAll(storeDir)
		return err
	}
	h := hub.New(1024)
	au := auth.New(time.Now)
	sh := share.New(time.Now)

	// 3. addresses
	addrs, _ := discovery.ListAddrs()
	publicBase := fmt.Sprintf("http://127.0.0.1:%d", port)
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ips = append(ips, a.IP)
		if a.Recommended {
			publicBase = fmt.Sprintf("http://%s:%d", a.IP, port)
		}
	}

	// 4. mDNS
	var mdnsHost, mdnsURL string
	stopMDNS, mdnsErr := announce(cfg.Name, port, ips)
	if mdnsErr == nil {
		mdnsHost = cfg.Name + ".local"
		mdnsURL = fmt.Sprintf("http://%s:%d", mdnsHost, port)
	}

	// 5. instance file
	secret, err := randomHex(32)
	if err != nil {
		if stopMDNS != nil {
			stopMDNS()
		}
		st.Close(false)
		return err
	}
	instPath := instance.DefaultPath()
	if err := instance.Write(instPath, instance.Info{PID: os.Getpid(), Port: port, Secret: secret}); err != nil {
		fmt.Fprintln(out, i18n.T(cfg.Lang, i18n.KeyWarnInstanceFile, err.Error()))
	}
	removeInstance := sync.OnceFunc(func() {
		// Only remove the file if it is still ours (a newer instance may
		// have replaced it).
		if i, err := instance.Read(instPath); err == nil && i.PID == os.Getpid() && i.Secret == secret {
			instance.Remove(instPath)
		}
	})
	defer removeInstance()

	// 6. server
	// The server calls the hook synchronously from the upload handler, so the
	// save is registered (saves.Add) before the response is written and thus
	// before srv.Shutdown returns. Shutdown then waits for in-flight saves
	// before the store dir is removed. A save arriving after that point (only
	// possible if Shutdown timed out) is skipped and logged.
	var (
		saveMu     sync.Mutex
		saveClosed bool
		saves      sync.WaitGroup
	)
	var onStored func(store.Item)
	if !cfg.NoSave {
		onStored = func(it store.Item) {
			if it.Type != store.KindFile || it.File == nil {
				return
			}
			saveMu.Lock()
			if saveClosed {
				saveMu.Unlock()
				fmt.Fprintln(out, i18n.T(cfg.Lang, i18n.KeyWarnAutosaveSkip, it.File.Name))
				return
			}
			saves.Add(1)
			saveMu.Unlock()
			go func() {
				defer saves.Done()
				home, err := os.UserHomeDir()
				if err == nil {
					_, err = autosave.Save(filepath.Join(st.Dir(), it.ID), it.File.Name, filepath.Join(home, "Downloads", "LanDrop"))
				}
				if err != nil {
					fmt.Fprintln(out, i18n.T(cfg.Lang, i18n.KeyWarnAutosave, it.File.Name, err.Error()))
				}
			}()
		}
	}
	handler := server.New(server.Deps{
		Store:        st,
		Hub:          h,
		Auth:         au,
		Share:        sh,
		NoAuth:       cfg.NoAuth,
		MaxSize:      cfg.MaxSize,
		PublicBase:   publicBase,
		MDNSURL:      mdnsURL,
		MDNSName:     cfg.Name,
		Dir:          cfg.Dir,
		LocalSecret:  secret,
		Hostname:     hostname(),
		OnFileStored: onStored,
		Web:          webui.Handler(),
		Now:          time.Now,
	})
	baseCtx, cancelBase := context.WithCancel(context.Background())
	defer cancelBase()
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// ReadTimeout/WriteTimeout stay 0: SSE and multi-GB uploads.
		BaseContext: func(net.Listener) context.Context { return baseCtx },
	}
	// Long-lived SSE handlers watch the request context; end them when
	// shutdown starts so Shutdown does not wait for its full timeout.
	srv.RegisterOnShutdown(cancelBase)

	// 7. sweeper
	sweepStop := make(chan struct{})
	var sweepDone sync.WaitGroup
	sweepDone.Add(1)
	go func() {
		defer sweepDone.Done()
		t := time.NewTicker(sweepInterval)
		defer t.Stop()
		for {
			select {
			case <-sweepStop:
				return
			case <-t.C:
				sweepOnce(st, sh, h)
			}
		}
	}()

	// 8. banner
	b := termui.BannerInfo{
		Lang:     cfg.Lang,
		Addrs:    addrs,
		Port:     port,
		MDNSHost: mdnsHost,
		MDNSErr:  mdnsErr,
		TokenURL: publicBase + "/?t=" + au.Token(),
		NoAuth:   cfg.NoAuth,
		GOOS:     runtime.GOOS,
		Version:  version.String(),
	}
	if !cfg.NoAuth {
		b.Code = au.Code()
	}
	printBanner(out, b)

	// 9. browser
	if !cfg.NoOpen {
		if err := openBrowser(fmt.Sprintf("http://127.0.0.1:%d/?t=%s", port, au.Token())); err != nil {
			fmt.Fprintln(out, i18n.T(cfg.Lang, i18n.KeyWarnOpenBrowser, err.Error()))
		}
	}

	// 10. serve
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	var runErr error
	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = err
		}
	}

	// 11. shutdown
	sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	if err := srv.Shutdown(sctx); err != nil {
		srv.Close()
	}
	cancel()
	close(sweepStop)
	sweepDone.Wait()
	if stopMDNS != nil {
		stopMDNS()
	}
	removeInstance()
	saveMu.Lock()
	saveClosed = true
	saveMu.Unlock()
	saves.Wait()
	st.Close(cfg.Keep)
	if cfg.Keep {
		fmt.Fprintln(out, i18n.T(cfg.Lang, i18n.KeyInfoKept, st.Dir()))
	} else {
		fmt.Fprintln(out, i18n.T(cfg.Lang, i18n.KeyInfoBye))
	}
	return runErr
}

// sweepOnce removes expired items, their share links and expired links.
func sweepOnce(st *store.Store, sh *share.Manager, h *hub.Hub) {
	for _, id := range st.Sweep() {
		sh.DropItem(id)
		h.Publish("item.deleted", map[string]string{"id": id})
	}
	sh.Sweep()
}

var osHostname = os.Hostname

// hostname names this machine for CLI-sent items, defaulting to "localhost".
func hostname() string {
	h, err := osHostname()
	if err != nil || h == "" {
		return "localhost"
	}
	return h
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
