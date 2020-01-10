package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
)

const tmdbAPIKey = "0cdacd9ab172ac6ff69c8d84b2c938a8"
const defaultMovieFormat = "{n} ({y})/{n} ({y}) {r}"
const defaultSeriesFormat = "{n}/Season.{s}/{n}.S{s}E{e}.{r}"

var addYearToSeries = map[string]bool{
	"The Flash":   true,
	"Doctor Who":  true,
	"Magnum P.I.": true,
	"Charmed":     true,
}

var actions = map[string]bool{
	"symlink":  true,
	"hardlink": true,
	"copy":     true,
}

var supportedCompressedExtensions = map[string]bool{
	".rar": true,
	".zip": true,
	".tar": true,
	".bz2": true,
	".gz":  true,
}

var supportedMusicExtensions = map[string]bool{
	".mp3":  true,
	".flac": true,
	".3pg":  true,
	".aac":  true,
	".alac": true,
	".opus": true,
	".ogg":  true,
	".wav":  true,
	".wmv":  true,
	".ape":  true,
}

var supportedVideoExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".mov":  true,
	".avi":  true,
	".webm": true,
	".wmv":  true,
	".mpg":  true,
	".mpeg": true,
}

var order = []string{"year", "season", "episode", "episodeAnime", "groupAnime", "audio", "resolution", "quality", "codec", "group", "proper", "repack", "hardcoded", "extended", "internal"}
var ignoreMovie = map[string]bool{
	"season":       true,
	"episode":      true,
	"episodeAnime": true,
	"groupAnime":   true,
}

var matchers = map[string]*regexp.Regexp{
	"year":         regexp.MustCompile("([\\[\\(]?((?:19[0-9]|20[01])[0-9])[\\]\\)]?)"),
	"season":       regexp.MustCompile("(?i)(s?([0-9]{1,2}))[EX]"),
	"episode":      regexp.MustCompile("(?i)[EX]([0-9]{2})(?:[^0-9]|$)"),
	"resolution":   regexp.MustCompile("(?i)(([0-9]{3,4}p))"),
	"audio":        regexp.MustCompile("MP3|DD5\\.?1|Dual[\\- ]Audio|LiNE|DTS|AAC(?:\\.?2\\.0)?|AC3(?:\\.5\\.1)?"),
	"quality":      regexp.MustCompile("((?:PPV\\.)?[HP]DTV|(?:HD)?CAM|B[DR]Rip|(?:HD-?)?TS|(?:PPV )?WEB-?DL(?: DVDRip)?|HDRip|DVDRip|DVDRIP|CamRip|W[EB]BRip|BluRay|DvDScr|hdtv|telesync)"),
	"codec":        regexp.MustCompile("(?i)xvid|x264|x265|h265|h\\.?264|h\\.?265"),
	"group":        regexp.MustCompile("(- ?([^-]+(?:-={[^-]+-?$)?))$"),
	"proper":       regexp.MustCompile("PROPER"),
	"repack":       regexp.MustCompile("REPACK"),
	"hardcoded":    regexp.MustCompile("HC"),
	"internal":     regexp.MustCompile("(?i)INTERNAL"),
	"extended":     regexp.MustCompile("(EXTENDED(:?.CUT)?)"),
	"episodeAnime": regexp.MustCompile("[-_ p.](\\d{2})[-_ (v\\[](\\d{2})?"),
	"groupAnime":   regexp.MustCompile("^(\\[\\w*\\])\\s(.*)\\s-"),
}

func configPath(path string) string {
	return filepath.Join(getHome(), ".config", "bis", path)
}

func defaultMovieFolder() string {
	return filepath.Join(getHome(), "media", "Movies")
}

func defaultSeriesFolder() string {
	return filepath.Join(getHome(), "media", "TV Shows")
}

func defaultExtractedFolder() string {
	return filepath.Join(getHome(), "media", "extracted")
}

func defaultMusicFolder() string {
	return filepath.Join(getHome(), "media", "Music")
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
