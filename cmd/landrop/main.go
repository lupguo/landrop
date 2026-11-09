// Command landrop shares files and text across devices on the local network.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/lupguo/landrop/internal/app"
	"github.com/lupguo/landrop/internal/config"
	"github.com/lupguo/landrop/internal/i18n"
	"github.com/lupguo/landrop/internal/sendcli"
	"github.com/lupguo/landrop/internal/version"
)

const usage = `Usage:
  landrop [flags]               start the LanDrop server
  landrop send <file>|-         send a file (or stdin) to the running server

Flags:
  --port int          listen port; tries +1 up to 10 times if busy (default 8848)
  --name string       mDNS name, reachable as <name>.local (default "drop")
  --dir string        directory to expose read-only for browsing (default none)
  --no-auth           disable the access code (anyone on the LAN can join)
  --ttl duration      item time to live (default 24h0m0s)
  --keep              keep temporary files on exit
  --max-size string   maximum file size (default "4GB")
  --no-save           do not autosave received files to ~/Downloads/LanDrop
  --no-open           do not open the browser on start
  --lang string       terminal language: en|zh|ja|ko (default "en", env LANDROP_LANG)
  --version           print version and exit
  -h, --help          show this help
`

func main() {
	if len(os.Args) > 1 && os.Args[1] == "send" {
		lang := os.Getenv("LANDROP_LANG")
		if !slices.Contains(i18n.Langs, lang) {
			lang = "en"
		}
		if err := sendcli.Run(os.Args[2:], os.Stdin, os.Stdout, lang); err != nil {
			os.Exit(1)
		}
		return
	}

	cfg, err := config.Parse(os.Args[1:], os.Getenv)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Print(usage)
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "landrop: %v\n\n%s", err, usage)
		os.Exit(2)
	}
	if cfg.ShowVersion {
		fmt.Println(version.String())
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, cfg, os.Stdout); err != nil {
		stop()
		fmt.Fprintf(os.Stderr, "landrop: %v\n", err)
		os.Exit(1)
	}
}
