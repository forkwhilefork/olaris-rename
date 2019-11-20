package main

import (
	"flag"
	"fmt"
	"github.com/mholt/archiver/v3"
	"github.com/ryanbradynd05/go-tmdb"

	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// NewEnv creates a new environment
func NewEnv(recursive bool, action string, movieFolder string, extractPath string, seriesFolder string, dryrun bool, tmdbLookup bool) *Env {
	return &Env{recursive: recursive, action: action, movieFolder: movieFolder, extractPath: extractPath, seriesFolder: seriesFolder, dryrun: dryrun, tmdbLookup: tmdbLookup}
}

// Env is a Standard environment with options
type Env struct {
	action       string
	movieFolder  string
	extractPath  string
	seriesFolder string
	dryrun       bool
	recursive    bool
	tmdbLookup   bool
}

type parsedFile struct {
	Year         string
	Season       string
	Episode      string
	ExternalName string
	CleanName    string
	Filepath     string
	Filename     string
	Extension    string
	Quality      string
	Resolution   string
	Group        string
	AnimeGroup   string
	IsSeries     bool
	IsMovie      bool
	IsMusic      bool
	ExternalID   int
}

func queryTmdb(p *parsedFile) error {
	var agent *tmdb.TMDb

	config := tmdb.Config{
		APIKey:   tmdbAPIKey,
		Proxies:  nil,
		UseProxy: false,
	}

	agent = tmdb.Init(config)

	var options = make(map[string]string)

	if p.Year != "" {
		options["first_air_date_year"] = p.Year
		options["year"] = p.Year
	}

	log.WithFields(log.Fields{"year": p.Year, "title": p.CleanName}).Debugln("Trying to locate data from TMDB")

	if p.IsSeries {
		searchRes, err := agent.SearchTv(p.CleanName, options)
		if err != nil {
			log.Warnln("Could not find a hit on TMDB")
			return err
		}

		if len(searchRes.Results) > 0 {
			tv := searchRes.Results[0] // Take the first result for now
			p.ExternalID = tv.ID
			p.ExternalName = tv.OriginalName
		} else {
			log.Debugln("No results")
		}

	} else if p.IsMovie {
		searchRes, err := agent.SearchMovie(p.CleanName, options)
		if err != nil {
			return err
		}

		if len(searchRes.Results) > 0 {
			mov := searchRes.Results[0] // Take the first result for now
			p.ExternalID = mov.ID
			p.ExternalName = mov.Title
		} else {
			log.Debugln("No results")
		}
	}
	return nil
}

// TargetName is the name the file should be renamed to, right now this is the FullName but if we want to make this smarter we can.
func (p *parsedFile) TargetName() string {
	var newName string

	if p.IsMovie {
		newName = defaultMovieFormat
	} else if p.IsSeries {
		newName = defaultSeriesFormat
		newName = strings.Replace(newName, "{s}", p.Season, -1)
		newName = strings.Replace(newName, "{e}", p.Episode, -1)
	} else {
		newName = p.Filename
	}

	if p.ExternalName != "" {
		newName = strings.Replace(newName, "{n}", p.ExternalName, -1)
	} else {
		newName = strings.Replace(newName, "{n}", p.CleanName, -1)
	}
	newName = strings.Replace(newName, "{r}", p.Resolution, -1)
	newName = strings.Replace(newName, "{q}", p.Quality, -1)
	newName = strings.Replace(newName, "{y}", p.Year, -1)
	newName = strings.Trim(newName, " ")

	return newName + p.Extension
}

// FullName is the original name of the file without the ful path
func (p *parsedFile) FullName() string {
	return p.Filename + p.Extension
}

func newParsedFile(filePath string, lookup bool) parsedFile {
	f := parsedFile{Filepath: filePath}
	f.Extension = filepath.Ext(filePath)
	filename := strings.TrimSuffix(filePath, f.Extension)
	filename = filepath.Base(filename)
	f.Filename = filename

	if supportedVideoExtensions[f.Extension] {
		for _, match := range order {
			res := matchers[match].FindStringSubmatch(filename)
			if len(res) > 0 {
				switch match {
				case "year":
					f.Year = res[2]
				case "season":
					f.Season = fmt.Sprintf("%02s", res[2])
				case "episode":
					f.Episode = fmt.Sprintf("%02s", res[1])
				case "quality":
					f.Quality = res[1]
				case "resolution":
					f.Resolution = res[2]
				case "groupAnime":
					f.AnimeGroup = res[1]
				case "episodeAnime":
					if f.Episode == "" {
						f.Episode = strings.Trim(res[0], " ")
						f.Season = "00"
					}
				}
			}
		}

		cleanName := strings.Replace(f.Filename, ".", " ", -1)

		if !f.IsMusic {
			for _, match := range order {
				res := matchers[match].FindStringSubmatch(cleanName)
				if len(res) > 0 {
					if match == "episode" {
						cleanName = strings.Replace(cleanName, res[0], " ", -1)
					} else if match == "season" {
						cleanName = strings.Replace(cleanName, res[1], " ", -1)
					} else if match == "groupAnime" {
						cleanName = strings.Replace(cleanName, res[1], " ", -1)
					} else {
						cleanName = matchers[match].ReplaceAllString(cleanName, "")
					}
				}
			}

			cleanName = strings.Trim(cleanName, " ")

			// Anime content is really weird, if we do this we might kill the name completely
			if f.AnimeGroup == "" {
				cleanName = regexp.MustCompile("\\s{2,}.*").ReplaceAllString(cleanName, "")
				cleanName = strings.Trim(cleanName, " - ")
				cleanName = strings.Title(cleanName)
			}
		}

		f.CleanName = cleanName
		if f.Episode == "" && f.Season == "" && f.Year != "" {
			f.IsMovie = true
		} else if f.Episode != "" && f.Season != "" {
			f.IsSeries = true
		} else if f.Episode == "" && f.Season == "" {
			log.WithFields(log.Fields{"file": f.Filename}).Warnln("Nothing sensible found, don't know how to continue")
		}

	} else if supportedMusicExtensions[f.Extension] {
		f.IsMusic = true
	}

	if lookup {
		queryTmdb(&f)
	}

	return f
}

func (p parsedFile) Act(targetFolder, action string) error {
	source, err := filepath.Abs(p.Filepath)
	if err != nil {
		return err
	}

	targetLocation := filepath.Join(targetFolder, p.TargetName())
	err = ensurePath(filepath.Dir(targetLocation))
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("Acting on file")

	if *dryrun == false {
		if _, err := os.Lstat(targetLocation); err == nil {
			log.Warnln("File already exists, doing nothing.")
			return nil
		}

		if action == "symlink" {
			source, err = filepath.Rel(filepath.Dir(targetLocation), source)
			if err != nil {
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
		}
	}
	return nil
}

func (e *Env) checkFile(filePath string) {
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
	if supportedCompressedExtensions[ext] {
		log.WithFields(log.Fields{"extension": ext, "file": filePath}).Println("Got a compressed file")

		err := archiver.Walk(filePath, func(file archiver.File) error {
			extension := filepath.Ext(file.Name())
			if supportedVideoExtensions[extension] {
				log.WithFields(log.Fields{"extension": ext, "filename": file.Name()}).Println("Extracting file and running new scan on the result")
				archiver.Extract(filePath, file.Name(), e.extractPath)
				target := strings.Replace(file.Name(), ext, "", -1)
				rec := e.recursive
				e.recursive = true
				e.StartRun(filepath.Join(e.extractPath, target))
				e.recursive = rec
			}
			return nil
		})
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Warnln("Received an error while looking at compressed data")
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
func (e *Env) StartRun(path string) {
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

func main() {
	flag.Parse()

	if *logToFile {
		lp := configFolderPath("bis.log")
		f, err := os.Create(lp)
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

	if *dryrun {
		log.Warnln("--dry-run is enabled, not touching files")
	}

	e := NewEnv(*recursive, *action, *movieFolder, *extractPath, *seriesFolder, *dryrun, *tmdbLookup)
	e.StartRun(*filePath)
}
