package main

import (
	"github.com/mholt/archiver/v3"

	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strings"
)

// NewApp creates a new environment
func NewApp(recursive bool, action string, movieFolder string, extractPath string, seriesFolder string, dryrun bool, tmdbLookup bool, skipExtracting bool) *App {
	return &App{recursive: recursive, action: action, movieFolder: movieFolder, extractPath: extractPath, seriesFolder: seriesFolder, dryrun: dryrun, tmdbLookup: tmdbLookup, skipExtracting: skipExtracting}
}

// App is a Standard environment with options
type App struct {
	action         string
	movieFolder    string
	extractPath    string
	seriesFolder   string
	dryrun         bool
	recursive      bool
	tmdbLookup     bool
	skipExtracting bool
}

func (e *App) walkRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			e.checkFile(path)
		}
		return nil
	})
}

func (e *App) checkFile(filePath string) {
	var err error

	log.WithFields(log.Fields{"filePath": filePath}).Debugln("Checking file")
	info, err := os.Stat(filePath)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Errorln("Received error while statting file.")
		return
	}
	if !info.Mode().IsRegular() {
		log.WithFields(log.Fields{"filePath": filePath}).Debugln("File is a directory, moving on.")
		return
	}

	ext := filepath.Ext(filePath)
	if supportedCompressedExtensions[ext] && !e.skipExtracting {
		log.WithFields(log.Fields{"extension": ext, "file": filePath}).Println("Got a compressed file")

		err := archiver.Walk(filePath, func(file archiver.File) error {
			extension := filepath.Ext(file.Name())
			if supportedVideoExtensions[extension] {
				log.WithFields(log.Fields{"extension": ext, "filename": file.Name()}).Println("Extracting file and running new scan on the result")
				archiver.Unarchive(filePath, e.extractPath)
				target := strings.Replace(file.Name(), ext, "", -1)
				rec := e.recursive
				e.recursive = true
				e.StartRun(filepath.Join(e.extractPath, target))
				e.recursive = rec
			}
			return nil
		})

		if err != nil {
			log.WithFields(log.Fields{"error": err}).Warnln("Received an error while looking through compressed data.")
		}
	}

	file := newParsedFile(filePath, e.tmdbLookup)

	if file.IsMovie {
		log.Debugln("File is a MovieFile")
		err = file.Act(e.movieFolder, e.action)
	} else if file.IsSeries {
		log.Debugln("File is a SeriesFile")
		err = file.Act(e.seriesFolder, e.action)
	} else if file.IsMusic {
		log.Debugln("File is a MusicFile, music is not supported yet.")
	}

	if err != nil {
		log.WithFields(log.Fields{"error": err}).Errorln("Received error while acting on parsed file")
	}

	log.WithFields(log.Fields{"filePath": filePath}).Debugln("Done checking file")

	return
}

// StartRun starts a identification run
func (e *App) StartRun(path string) {
	fi, err := os.Stat(path)
	if err != nil {
		panic(err)
	}

	if fi.IsDir() {
		if e.recursive == false {
			log.Infof("Scanning non-recursive path '%s'", path)
			files, err := filepath.Glob(filepath.Join(path, "*"))
			if err != nil {
				log.WithFields(log.Fields{"error": err}).Errorln("Received error while discovering files in folder")
			}
			for _, f := range files {
				e.checkFile(f)
			}
		} else if e.recursive {
			log.Infof("Scanning path '%s' recursively", path)
			e.walkRecursive(path + "/")
		}
	} else {
		e.checkFile(path)
	}
}
