package identify

import (
	"fmt"
	"strconv"

	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	log "github.com/sirupsen/logrus"
)

type ParsedFile struct {
	Year         string
	Season       string
	Episode      string
	EpisodeName  string
	ExternalName string
	CleanName    string
	Filepath     string
	Filename     string
	Extension    string
	Quality      string
	Resolution   string
	TechnicalInfo string
	Group        string
	AnimeGroup   string
	IsSeries     bool
	IsMovie      bool
	ExternalID   int
	OriginalFile string
	Options      Options

	hasYearAsSeason bool
}

func (p *ParsedFile) String() string {
	return fmt.Sprintf("Year: %s, Season: %s, Episode: %s, EpisodeName: %s, TechnicalInfo: %s, Name: %s, Movie: %v, Series: %v", p.Year, p.Season, p.Episode, p.EpisodeName, p.TechnicalInfo, p.CleanName, p.IsMovie, p.IsSeries)
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
						f.hasYearAsSeason = true
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

		// Extract technical info before cleaning process removes it
		f.extractTechnicalInfo()

		cleanName := strings.Replace(f.Filename, ".", " ", -1)

		if !f.IsMovie {
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
							oldName := cleanName
							cleanName = matchers[match].ReplaceAllString(cleanName, " ")
							if len(strings.TrimRight(cleanName, " ")) < 2 {
								log.WithFields(log.Fields{"matcher": match, "newName": cleanName, "oldName": oldName}).Debugln("The match we just did made the name of the content smaller than two characters, we are going to assume something went wrong and reverting to the previous name.")
								cleanName = oldName
							}
						}
					}
				}
			}

			cleanName = strings.Trim(cleanName, " ")

			// Anime content is really weird, if we do this we might kill the name completely
			if f.AnimeGroup == "" {
				log.WithField("cleanName", cleanName).Debugln("Probably not Anime so cleaning a bit more.")
				cleanName = regexp.MustCompile(`\s{2,}.*`).ReplaceAllString(cleanName, "")
				//cleanName = strings.Trim(cleanName, " -")
				cleanName = cases.Title(language.English).String(cleanName)
			}
		}

		cleanName = strings.Replace(cleanName, ":", "", -1)

		f.CleanName = cleanName
	} else {
		return f
	}

	if opts.Lookup {
		queryTmdb(&f)
	}

	if f.ExternalID > 0 && f.hasYearAsSeason {
		// Translate season as year to season number
		agent := initAgent()
		details, err := agent.GetTvInfo(f.ExternalID, nil)
		if err != nil {
			log.Errorln("Could not locate TV even though we just found an external ID, this shouldn't be possible. Error:", err)
		}
		couldTranslate := false
		for _, s := range details.Seasons {
			if s.Name == fmt.Sprintf("Season %s", f.Season) {
				log.Debugln("Found a match for the season name, using season number.")
				f.Season = strconv.Itoa(s.SeasonNumber)
				couldTranslate = true
				break
			}
		}
		if !couldTranslate {
			log.Warnln("Could not translate season as year to normal season :-(")
		}
	} else if f.hasYearAsSeason && !f.Options.Lookup {
		log.Warnln("Found an episode that has a year as season but lookup is disabled so not translating season as year to normal season.")
	}

	if addYearToSeries[f.CleanName] && f.Year != "" {
		log.WithFields(log.Fields{"year": f.Year, "name": f.CleanName}).Debugln("Found seriesname that has multiple series with the same name but different years so adding the year into the final name.")
		f.CleanName = fmt.Sprintf("%s (%s)", f.CleanName, f.Year)
	}

	// Windows really hates colons, so lets strip them out.
	f.CleanName = strings.Replace(f.CleanName, ":", "", -1)

	log.WithField("cleanName", f.String()).Infoln("Done parsing filename.")

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
	agent := initAgent()

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
			
			// Fetch episode name if we have season and episode information
			if p.Season != "" && p.Episode != "" {
				seasonNum, err1 := strconv.Atoi(p.Season)
				episodeNum, err2 := strconv.Atoi(p.Episode)
				if err1 == nil && err2 == nil {
					episodeInfo, err := agent.GetTvEpisodeInfo(tv.ID, seasonNum, episodeNum, nil)
					if err == nil && episodeInfo.Name != "" {
						// Clean episode name for filesystem compatibility
						p.EpisodeName = strings.Replace(episodeInfo.Name, ":", "", -1)
						p.EpisodeName = strings.Replace(p.EpisodeName, "/", "-", -1)
						p.EpisodeName = strings.Replace(p.EpisodeName, "\\", "-", -1)
						log.WithFields(log.Fields{"episodeName": p.EpisodeName, "season": seasonNum, "episode": episodeNum}).Debugln("Found episode name from TMDB")
					} else {
						log.WithFields(log.Fields{"season": seasonNum, "episode": episodeNum, "error": err}).Debugln("Could not fetch episode name from TMDB")
					}
				}
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
		newName = strings.Replace(newName, "{t}", p.EpisodeName, -1)
	} else {
		newName = p.Filename
	}

	newName = strings.Replace(newName, "{n}", p.CleanName, -1)

	newName = strings.Replace(newName, "{r}", p.Resolution, -1)
	newName = strings.Replace(newName, "{q}", p.Quality, -1)
	newName = strings.Replace(newName, "{y}", p.Year, -1)
	newName = strings.Replace(newName, "{i}", p.TechnicalInfo, -1)
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

// EpisodeNum converts Episode to integer, mainly for Olaris-Server support
func (p *ParsedFile) EpisodeNum() (episodeNum int) {
	episodeNum, err := strconv.Atoi(p.Episode)
	if err != nil {
		log.Warnln("Received error when converting episode to int", err)
	}

	return episodeNum
}

// EpisodeNum converts Season to integer, mainly for Olaris-Server support
func (p *ParsedFile) SeasonNum() (seasonNum int) {
	seasonNum, err := strconv.Atoi(p.Season)
	if err != nil {
		log.Warnln("Received error when converting season to int", err)
	}

	return seasonNum
}

// extractTechnicalInfo extracts technical information (resolution, quality, codec, etc.) from filename
func (p *ParsedFile) extractTechnicalInfo() {
	filename := p.Filename
	
	// List of technical patterns to look for, in order of likelihood
	technicalPatterns := []string{"resolution", "quality", "codec", "audio"}
	
	earliestMatch := len(filename)
	matchStart := -1
	
	// Find the earliest occurrence of any technical pattern
	for _, pattern := range technicalPatterns {
		if matcher, exists := matchers[pattern]; exists {
			match := matcher.FindStringIndex(filename)
			if match != nil && match[0] < earliestMatch {
				earliestMatch = match[0]
				matchStart = match[0]
			}
		}
	}
	
	// If we found technical info, extract everything from that point onward
	if matchStart != -1 {
		// Find the start of the technical info by looking backwards for a separator
		techStart := matchStart
		for i := matchStart - 1; i >= 0; i-- {
			if filename[i] == '.' || filename[i] == '-' || filename[i] == ' ' {
				techStart = i + 1
				break
			}
		}
		
		if techStart < len(filename) {
			technicalInfo := filename[techStart:]
			// Preserve the technical info exactly as it appears - no modifications
			p.TechnicalInfo = technicalInfo
			log.WithFields(log.Fields{"technicalInfo": p.TechnicalInfo}).Debugln("Extracted technical info")
		}
	}
}
