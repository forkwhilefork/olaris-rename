package main

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	log "github.com/sirupsen/logrus"
	"gitlab.com/olaris/olaris-rename/identify"
)

// NewApp creates a new environment
func NewApp(recursive bool, action string, movieFolder string, seriesFolder string, dryrun bool, tmdbLookup bool, minFileSize string, forceMovie bool, forceSeries bool) *App {
	return &App{recursive: recursive, action: action, movieFolder: movieFolder, seriesFolder: seriesFolder, dryrun: dryrun, tmdbLookup: tmdbLookup, minFileSize: minFileSize, forceMovie: forceMovie, forceSeries: forceSeries}
}

// App is a Standard environment with options
type App struct {
	action       string
	movieFolder  string
	seriesFolder string
	minFileSize  string
	dryrun       bool
	recursive    bool
	tmdbLookup   bool
	forceMovie   bool
	forceSeries  bool
}

var actions = map[string]bool{
	"rename":   true,
	"symlink":  true,
	"hardlink": true,
	"copy":     true,
	"move":     true,
}

func defaultMovieFolder() string {
	return filepath.Join(getHome(), "media", "Movies")
}

func defaultSeriesFolder() string {
	return filepath.Join(getHome(), "media", "TV Shows")
}

func defaultConfigFolder() string {
	p := filepath.Join(getHome(), ".config", "olaris-renamer")
	ensurePath(p)
	return p
}

func configFolderPath(p string) string {
	path := filepath.Join(defaultConfigFolder(), p)
	return path
}

func getHome() string {
	usr, err := user.Current()
	if err != nil {
		panic(fmt.Sprintf("Failed to determine user's home directory, error: '%s'\n", err.Error()))
	}
	return usr.HomeDir
}

// ensurePath ensures the given filesystem path exists, if not it will create it.
func ensurePath(pathName string) error {
	if _, err := os.Stat(pathName); os.IsNotExist(err) {
		log.WithFields(log.Fields{"pathName": pathName}).Debugln("Creating folder as it does not exist yet.")
		err = os.MkdirAll(pathName, 0755)
		if err != nil {
			log.WithFields(log.Fields{"pathName": pathName}).Debugln("Could not create path.")
			return err
		}
	}
	return nil
}

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

func (e *App) minFileSizeBytes() int64 {
	mb, err := strconv.Atoi(e.minFileSize)
	if err != nil {
		log.Warnln("could not parse given minFileSize, returning default one")
		return 2 * 1000 * 1000
	}
	return int64(mb) * 1000 * 1000
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
	log.WithFields(log.Fields{"filePath": filePath}).Debugln("checking file")

	ext := filepath.Ext(filePath)

	info, err := os.Stat(filePath)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "filePath": filePath}).Errorln("received error while statting file.")
		return
	}
	if !info.Mode().IsRegular() {
		log.WithFields(log.Fields{"filePath": filePath}).Debugln("File is a directory, moving on.")
		return
	}

	if identify.SupportedVideoExtensions[ext] {
		if info.Size() < e.minFileSizeBytes() {
			log.WithFields(log.Fields{"filePath": filePath, "minSize": e.minFileSizeBytes(), "size": info.Size()}).Warnln("file is smaller then the given limit, not processing.")
			return
		}
	}

	file := identify.NewParsedFile(filePath, identify.Options{Lookup: e.tmdbLookup, MovieFormat: *movieFormat, SeriesFormat: *seriesFormat, ForceMovie: e.forceMovie, ForceSeries: e.forceSeries, DryRun: e.dryrun})

	if file.IsMovie {
		log.Debugln("File is a MovieFile")
		if e.action == "rename" {
			err = actRename(file, e.action)
		} else {
			err = act(file, e.movieFolder, e.action)
		}
	} else if file.IsSeries {
		log.Debugln("File is a SeriesFile")
		if e.action == "rename" {
			err = actRename(file, e.action)
		} else {
			err = act(file, e.seriesFolder, e.action)
		}
	}

	if err != nil {
		log.WithFields(log.Fields{"error": err}).Errorln("Received error while acting on parsed file")
	}

	log.WithFields(log.Fields{"filePath": filePath}).Debugln("Done checking file")
}

func actRename(p identify.ParsedFile, action string) error {
	source, err := filepath.Abs(p.SourcePath())
	if err != nil {
		return err
	}

	// For rename action, we rename in the same directory as the source
	// Extract just the filename part from TargetName (strip any folder structure)
	sourceDir := filepath.Dir(source)
	targetFullName := p.TargetName()
	targetFileName := filepath.Base(targetFullName) // Get just the filename without folder structure
	targetLocation := filepath.Join(sourceDir, targetFileName)

	if !p.Options.DryRun {
		log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("Acting on file")
		if _, err := os.Lstat(targetLocation); err == nil {
			log.Warnln("File already exists, doing nothing.")
			return nil
		}

		err := os.Rename(source, targetLocation)
		if err != nil {
			return err
		}
	} else {
		log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("--dry-run enabled, not acting on file")
	}

	return nil
}

func act(p identify.ParsedFile, targetFolder, action string) error {
	source, err := filepath.Abs(p.SourcePath())
	if err != nil {
		return err
	}

	targetLocation := filepath.Join(targetFolder, p.TargetName())

	if !p.Options.DryRun {
		err = ensurePath(filepath.Dir(targetLocation))
		if err != nil {
			return err
		}

		log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("Acting on file")
		if _, err := os.Lstat(targetLocation); err == nil {
			log.Warnln("File already exists, doing nothing.")
			return nil
		}

		if action == "symlink" {
			source, err = filepath.EvalSymlinks(source)
			if err != nil {
				log.WithFields(log.Fields{"targetLocation": filepath.Dir(targetLocation), "source": source, "err": err}).Debugln("error during symlink evaluation")
				return err
			}

			log.WithFields(log.Fields{"source": source, "target": targetLocation}).Debugln("Evaling symlinks")
			source, err = filepath.Rel(filepath.Dir(targetLocation), source)

			if err != nil {
				log.WithFields(log.Fields{"targetLocation": filepath.Dir(targetLocation), "source": source, "err": err}).Debugln("error during Rel call")
				return err
			}

			log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("Using relative path for symlinks.")

			err = os.Symlink(source, targetLocation)
			if err != nil {
				return err
			}
		} else if action == "hardlink" {
			err = os.Link(source, targetLocation)
			if err != nil {
				return err
			}
		} else if action == "copy" {
			err := copyFileContents(source, targetLocation)
			if err != nil {
				return err
			}
		} else if action == "move" {
			err := os.Rename(source, targetLocation)
			if err != nil {
				return err
			}
		}
	} else {
		log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("--dry-run enabled, not acting on file")
	}

	return nil
}

// StartRun starts a identification run
func (e *App) StartRun(path string) {
	fi, err := os.Stat(path)
	if err != nil {
		log.WithFields(log.Fields{"path": path, "error": err}).Errorf("could not open file")
		return
	}

	if fi.IsDir() {
		if !e.recursive {
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
