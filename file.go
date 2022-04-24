package main

import (
	"fmt"

	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ryanbradynd05/go-tmdb"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	log "github.com/sirupsen/logrus"
)

type ParsedFile struct {
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
	OriginalFile string
}

func NewParsedFile(filePath string, lookup bool, originalFile string) ParsedFile {
	f := ParsedFile{Filepath: filePath, OriginalFile: originalFile}
	f.Extension = filepath.Ext(filePath)
	filename := strings.TrimSuffix(filePath, f.Extension)
	filename = filepath.Base(filename)
	f.Filename = filename
	log.WithFields(log.Fields{"file": f.Filename}).Debugln("Checking file")

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
			log.WithFields(log.Fields{"cleanName": cleanName, "year": f.Year, "episode": f.Episode, "season": f.Season}).Debugln("Pre-parsing done, initial result.")
			if f.Episode == "" && f.Season == "" && f.Year != "" {
				f.IsMovie = true
				log.Debugln("Identified file as a movie")
			} else if f.Episode != "" && f.Season != "" {
				f.IsSeries = true
				log.Debugln("Identified file as an episode")
			} else {
				fileParent := filepath.Base(filepath.Dir(filePath))
				if fileParent != "" && originalFile == "" && fileParent != "." {
					log.WithFields(log.Fields{"file": f.Filename, "filePath": filePath, "fileParent": fileParent}).Warnln("Nothing sensible found, trying again with parent.")
					return NewParsedFile(fileParent+f.Extension, lookup, filePath)
				}
			}

			for _, match := range order {
				res := matchers[match].FindStringSubmatch(cleanName)
				if len(res) > 0 {
					// We don't need to remove Episode and Season information from movies, so let's exclude some properties
					if (f.IsMovie && !ignoreMovie[match]) || f.IsSeries {
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
			}

			cleanName = strings.Trim(cleanName, " ")

			// Anime content is really weird, if we do this we might kill the name completely
			if f.AnimeGroup == "" {
				cleanName = regexp.MustCompile(`\s{2,}.*`).ReplaceAllString(cleanName, "")
				cleanName = strings.Trim(cleanName, " -")
				cleanName = cases.Title(language.English).String(cleanName)
			}
		}

		cleanName = strings.Replace(cleanName, ":", "", -1)
		f.CleanName = cleanName
	} else if supportedMusicExtensions[f.Extension] {
		f.IsMusic = true
		return f
	} else {
		return f
	}

	if lookup {
		queryTmdb(&f)
	}

	if addYearToSeries[f.CleanName] && f.Year != "" {
		log.WithFields(log.Fields{"year": f.Year, "name": f.CleanName}).Debugln("Found series which requires year to be added")
		f.CleanName = fmt.Sprintf("%s (%s)", f.CleanName, f.Year)
	}

	// Windows really hates colons, so lets strip them out.
	f.CleanName = strings.Replace(f.CleanName, ":", "", -1)

	return f
}

// SourcePath returns the originfile if one is available otherwise does the most recursed hit.
func (p *ParsedFile) SourcePath() string {
	log.Println(p.Filepath)
	log.Println(p.OriginalFile)
	if p.OriginalFile == "" {
		return p.Filepath
	} else {
		return p.OriginalFile
	}
}

func queryTmdb(p *ParsedFile) error {
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
			log.WithFields(log.Fields{"name": p.CleanName, "error": err}).Warnln("Got an error from TMDB")
			return err
		}

		if len(searchRes.Results) > 0 {
			tv := searchRes.Results[0] // Take the first result for now
			p.ExternalID = tv.ID
			p.ExternalName = tv.Name
			p.CleanName = tv.Name
			if tv.FirstAirDate != "" && p.Year == "" {
				p.Year = strings.Split(tv.FirstAirDate, "-")[0]
			}
		} else {
			log.Debugln("No results")
		}

	} else if p.IsMovie {
		searchRes, err := agent.SearchMovie(p.CleanName, options)
		if err != nil {
			log.WithFields(log.Fields{"name": p.CleanName, "error": err}).Warnln("Got an error from TMDB")
			return err
		}

		if len(searchRes.Results) > 0 {
			mov := searchRes.Results[0] // Take the first result for now
			p.ExternalID = mov.ID
			p.ExternalName = mov.Title
			p.CleanName = mov.Title
		} else {
			log.Debugln("No results")
		}
	}

	return nil
}

// TargetName is the name the file should be renamed to
func (p *ParsedFile) TargetName() string {
	var newName string

	if p.IsMovie {
		newName = *movieFormat
	} else if p.IsSeries {
		newName = *seriesFormat
		newName = strings.Replace(newName, "{s}", p.Season, -1)
		newName = strings.Replace(newName, "{e}", p.Episode, -1)
	} else {
		newName = p.Filename
	}

	newName = strings.Replace(newName, "{n}", p.CleanName, -1)

	newName = strings.Replace(newName, "{r}", p.Resolution, -1)
	newName = strings.Replace(newName, "{q}", p.Quality, -1)
	newName = strings.Replace(newName, "{y}", p.Year, -1)
	newName = strings.Trim(newName, " ")

	return newName + p.Extension
}

// FullName is the original name of the file without the ful path
func (p *ParsedFile) FullName() string {
	return p.Filename + p.Extension
}

func (p ParsedFile) Act(targetFolder, action string) error {
	source, err := filepath.Abs(p.SourcePath())
	if err != nil {
		return err
	}

	targetLocation := filepath.Join(targetFolder, p.TargetName())

	if !*dryrun {
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
