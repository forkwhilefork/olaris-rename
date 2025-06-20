package main

import (
	"flag"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	flag.Parse()

	if *logToFile {
		lp := configFolderPath("olaris-rename.log")
		f, err := os.OpenFile(lp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		mw := io.MultiWriter(os.Stdout, f)
		log.SetOutput(mw)
		defer f.Close()
	}

	if *verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	if *filePath == "" {
		log.Errorln("--filepath is a required argument.")
		flag.PrintDefaults()
		return
	}

	if !actions[*action] {
		log.Errorf("Unknown --action '%s'", *action)
		flag.PrintDefaults()
		return
	}

	var modes = map[string]bool{
		"dry-run":     true,
		"interactive": true,
		"force":       true,
	}

	if !modes[*mode] {
		log.Errorf("Unknown --mode '%s', valid options are: dry-run, interactive, force", *mode)
		flag.PrintDefaults()
		return
	}

	if *mode == "dry-run" {
		log.Warnln("Mode is set to dry-run, not touching files")
	} else if *mode == "interactive" {
		log.Infoln("Mode is set to interactive, will prompt for confirmation")
	} else if *mode == "force" {
		log.Warnln("Mode is set to force, will execute without confirmation")
	}

	e := NewApp(*recursive, *action, *movieFolder, *seriesFolder, *mode, *tmdbLookup, *minFileSize, *forceMovie, *forceSeries)
	e.StartRun(*filePath)
}
