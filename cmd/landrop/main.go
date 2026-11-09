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

// envLang is LANDROP_LANG if it is a supported language, else "en".
func envLang() string {
	if l := os.Getenv("LANDROP_LANG"); slices.Contains(i18n.Langs, l) {
		return l
	}
	return "en"
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "send" {
		if err := sendcli.Run(os.Args[2:], os.Stdin, os.Stdout, envLang()); err != nil {
			os.Exit(1) // sendcli already printed the localized message
		}
		return
	}

	cfg, err := config.Parse(os.Args[1:], os.Getenv)
	if errors.Is(err, flag.ErrHelp) {
		config.Usage(os.Stdout, envLang())
		return
	}
	if err != nil {
		lang := envLang()
		fmt.Fprintln(os.Stderr, i18n.T(lang, i18n.KeyCLIError, err.Error()))
		fmt.Fprintln(os.Stderr)
		config.Usage(os.Stderr, lang)
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
		fmt.Fprintln(os.Stderr, i18n.T(cfg.Lang, i18n.KeyCLIError, err.Error()))
		os.Exit(1)
	}
}
