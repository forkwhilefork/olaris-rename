package identify

import (
	"fmt"

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
	Options      Options
}

func (p *ParsedFile) String() string {
	return fmt.Sprintf("Year: %s, Season: %s, Episode: %s, Name: %s, Movie: %v, Series: %v", p.Year, p.Season, p.Episode, p.CleanName, p.IsMovie, p.IsSeries)
}

type Options struct {
	Lookup       bool
	ForceMovie   bool
	ForceSeries  bool
	OriginalFile string
	MovieFormat  string
	SeriesFormat string
	DryRun       bool
}

func (p *Options) String() string {
	return fmt.Sprintf("Lookup: %v, ForceMovie: %v, ForceSeries: %v, OriginalFile: %s, MovieFormat: %s, SeriesFormat: %s, DryRun: %v", p.Lookup, p.ForceMovie, p.ForceSeries, p.OriginalFile, p.MovieFormat, p.SeriesFormat, p.DryRun)
}

func GetDefaultOptions() Options {
	return getOpts([]Options{})
}

func getOpts(o []Options) (opts Options) {
	if len(o) == 0 {
		opts = Options{}
	} else {
		opts = o[0]
	}
	if opts.MovieFormat == "" {
		opts.MovieFormat = DefaultMovieFormat
	}
	if opts.SeriesFormat == "" {
		opts.SeriesFormat = DefaultSeriesFormat
	}
	return opts
}

func NewParsedFile(filePath string, o ...Options) ParsedFile {
	opts := getOpts(o)
	log.WithField("options", opts.String()).Debugln("Parsing filename with options")
	f := ParsedFile{Filepath: filePath, OriginalFile: opts.OriginalFile, Options: opts}
	f.Extension = filepath.Ext(filePath)
	filename := strings.TrimSuffix(filePath, f.Extension)
	filename = filepath.Base(filename)
	f.Filename = filename
	log.WithFields(log.Fields{"file": f.Filename}).Debugln("Checking file")

	if SupportedVideoExtensions[f.Extension] {
		for _, match := range order {
			res := matchers[match].FindStringSubmatch(filename)
			if len(res) > 0 {
				switch match {
				case "yearAsSeason":
					if len(res) > 1 {
						log.WithField("year", res[2]).Debugln("Found year as season.")
						f.Season = res[2]
					}
				case "year":
					if f.Season == res[2] {
						log.Warnln("We found a year that is the same as the season, to prevent issues with looking up the wrong year we are ignoring the found year.")
					} else {
						f.Year = res[2]
					}
				case "season":
					if f.Season == "" {
						f.Season = fmt.Sprintf("%02s", res[2])
					} else {
						log.Debugln("We already have found a season earlier so skipping the normal season match.")
					}
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
			if opts.ForceMovie || (f.Episode == "" && f.Season == "" && f.Year != "") {
				f.IsMovie = true
				log.Debugln("Identified file as a movie")
			} else if opts.ForceSeries || (f.Episode != "" && f.Season != "") {
				f.IsSeries = true
				log.Debugln("Identified file as an episode")
			} else {
				fileParent := filepath.Base(filepath.Dir(filePath))
				if fileParent != "" && opts.OriginalFile == "" && fileParent != "." {
					log.WithFields(log.Fields{"file": f.Filename, "filePath": filePath, "fileParent": fileParent}).Warnln("Nothing sensible found, trying again with parent.")
					opts.OriginalFile = filePath
					return NewParsedFile(fileParent+f.Extension, opts)
				}
			}
			log.Debugln("Starting actual matching.")

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
							cleanName = matchers[match].ReplaceAllString(cleanName, " ")
						}
					}
				}
			}

			cleanName = strings.Trim(cleanName, " ")

			// Anime content is really weird, if we do this we might kill the name completely
			if f.AnimeGroup == "" {
				log.WithField("cleanName", cleanName).Debugln("Probably not Anime so cleaning a bit more.")
				cleanName = regexp.MustCompile(`\s{2,}.*`).ReplaceAllString(cleanName, "")
				cleanName = strings.Trim(cleanName, " -")
				cleanName = cases.Title(language.English).String(cleanName)
			}
		}

		cleanName = strings.Replace(cleanName, ":", "", -1)

		f.CleanName = cleanName
	} else if SupportedMusicExtensions[f.Extension] {
		f.IsMusic = true
		return f
	} else {
		return f
	}

	if opts.Lookup {
		queryTmdb(&f)
	}

	if addYearToSeries[f.CleanName] && f.Year != "" {
		log.WithFields(log.Fields{"year": f.Year, "name": f.CleanName}).Debugln("Found seriesname that has multiple series with the same name but different years so adding the year into the final name.")
		f.CleanName = fmt.Sprintf("%s (%s)", f.CleanName, f.Year)
	}

	// Windows really hates colons, so lets strip them out.
	f.CleanName = strings.Replace(f.CleanName, ":", "", -1)

	log.WithField("cleanName", f.String()).Debugln("Final result.")

	return f
}

// SourcePath returns the originfile if one is available otherwise does the most recursed hit.
func (p *ParsedFile) SourcePath() string {
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
			log.Debugln("TV:", tv)
			p.ExternalID = tv.ID
			p.ExternalName = tv.Name
			p.CleanName = tv.Name
			if tv.FirstAirDate != "" && p.Year == "" {
				p.Year = strings.Split(tv.FirstAirDate, "-")[0]
			}
		} else {
			log.Debugln("No results found on TMDB")
		}

	} else if p.IsMovie {
		searchRes, err := agent.SearchMovie(p.CleanName, options)
		if err != nil {
			log.WithFields(log.Fields{"name": p.CleanName, "error": err}).Warnln("Got an error from TMDB")
			return err
		}

		if len(searchRes.Results) > 0 {

			mov := searchRes.Results[0] // Take the first result for now
			log.Debugln("Movie:", mov)

			p.ExternalID = mov.ID
			p.ExternalName = mov.Title
			p.CleanName = mov.Title

		} else {
			log.Debugln("No results found on TMDB")
		}
	}

	log.WithFields(log.Fields{"externalID": p.ExternalID, "externalName": p.ExternalName}).Debugln("Received TMDB results.")

	return nil
}

// TargetName is the name the file should be renamed to
func (p *ParsedFile) TargetName() string {
	var newName string

	if p.IsMovie {
		newName = p.Options.MovieFormat
	} else if p.IsSeries {
		newName = p.Options.SeriesFormat
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

	// Sometimes we end up with an extra dot somehow. This should remove the extra dot
	// I'm tired, this can probably be done better...
	if (string(newName[len(newName)-1])) == "." {
		newName = newName[0 : len(newName)-1]
	}

	return newName + p.Extension
}

// FullName is the original name of the file without the ful path
func (p *ParsedFile) FullName() string {
	return p.Filename + p.Extension
}
