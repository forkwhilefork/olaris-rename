package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"gitlab.com/olaris/olaris-rename/identify"
)

// NewApp creates a new environment
func NewApp(recursive bool, action string, movieFolder string, seriesFolder string, mode string, tmdbLookup bool, minFileSize string, forceMovie bool, forceSeries bool) *App {
	return &App{recursive: recursive, action: action, movieFolder: movieFolder, seriesFolder: seriesFolder, mode: mode, tmdbLookup: tmdbLookup, minFileSize: minFileSize, forceMovie: forceMovie, forceSeries: forceSeries}
}

// App is a Standard environment with options
type App struct {
	action       string
	movieFolder  string
	seriesFolder string
	minFileSize  string
	mode         string
	recursive    bool
	tmdbLookup   bool
	forceMovie   bool
	forceSeries  bool
}

// PlannedOperation represents a file operation that will be performed
type PlannedOperation struct {
	SourcePath string
	TargetPath string
	Action     string
	IsMovie    bool
	IsSeries   bool
	File       identify.ParsedFile
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

// collectPlannedOperations collects all the file operations that would be performed
func (e *App) collectPlannedOperations(path string) ([]PlannedOperation, error) {
	var operations []PlannedOperation
	
	// Temporarily set mode to dry-run to collect operations without executing
	originalMode := e.mode
	e.mode = "dry-run"
	defer func() { e.mode = originalMode }()
	
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		if !e.recursive {
			files, err := filepath.Glob(filepath.Join(path, "*"))
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				ops := e.collectFileOperations(f)
				operations = append(operations, ops...)
			}
		} else if e.recursive {
			err := filepath.Walk(path+"/", func(filePath string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.Mode().IsRegular() {
					ops := e.collectFileOperations(filePath)
					operations = append(operations, ops...)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	} else {
		operations = e.collectFileOperations(path)
	}
	
	return operations, nil
}

// collectFileOperations collects the planned operations for a single file
func (e *App) collectFileOperations(filePath string) []PlannedOperation {
	var operations []PlannedOperation
	
	ext := filepath.Ext(filePath)
	info, err := os.Stat(filePath)
	if err != nil {
		return operations
	}
	
	if !info.Mode().IsRegular() {
		return operations
	}

	if identify.SupportedVideoExtensions[ext] {
		if info.Size() < e.minFileSizeBytes() {
			return operations
		}
	}

	file := identify.NewParsedFile(filePath, identify.Options{
		Lookup: e.tmdbLookup, 
		MovieFormat: *movieFormat, 
		SeriesFormat: *seriesFormat, 
		ForceMovie: e.forceMovie, 
		ForceSeries: e.forceSeries, 
		Mode: "dry-run", // Force dry-run mode for collection
	})

	if file.IsMovie || file.IsSeries {
		var targetPath string
		
		if e.action == "rename" {
			source, err := filepath.Abs(file.SourcePath())
			if err != nil {
				return operations
			}
			sourceDir := filepath.Dir(source)
			targetFullName := file.TargetName()
			targetFileName := filepath.Base(targetFullName)
			targetPath = filepath.Join(sourceDir, targetFileName)
		} else {
			var targetFolder string
			if file.IsMovie {
				targetFolder = e.movieFolder
			} else {
				targetFolder = e.seriesFolder
			}
			targetPath = filepath.Join(targetFolder, file.TargetName())
		}
		
		operations = append(operations, PlannedOperation{
			SourcePath: file.SourcePath(),
			TargetPath: targetPath,
			Action:     e.action,
			IsMovie:    file.IsMovie,
			IsSeries:   file.IsSeries,
			File:       file,
		})
	}
	
	return operations
}

// promptForConfirmation displays the planned operations and asks for user confirmation
func (e *App) promptForConfirmation(operations []PlannedOperation) bool {
	if len(operations) == 0 {
		fmt.Println("No files to process.")
		return false
	}
	
	fmt.Printf("\nFound %d file(s) to process:\n\n", len(operations))
	
	// Get current working directory for relative path calculation
	cwd, err := os.Getwd()
	if err != nil {
		// If we can't get cwd, fall back to absolute paths
		cwd = ""
	}
	
	for i, op := range operations {
		fmt.Printf("%d. %s\n", i+1, op.Action)
		
		// Convert to relative paths
		fromPath := op.SourcePath
		toPath := op.TargetPath
		
		if cwd != "" {
			if relFrom, err := filepath.Rel(cwd, op.SourcePath); err == nil {
				fromPath = relFrom
			}
			if relTo, err := filepath.Rel(cwd, op.TargetPath); err == nil {
				toPath = relTo
			}
		}
		
		fmt.Printf("   From: %s\n", fromPath)
		fmt.Printf("   To:   %s\n", toPath)
		fmt.Println()
	}
	
	fmt.Print("Do you want to proceed with these operations? (y/N): ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		log.WithError(err).Errorln("Error reading user input")
		return false
	}
	
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// executeOperations performs the actual file operations
func (e *App) executeOperations(operations []PlannedOperation) {
	for _, op := range operations {
		// Update the ParsedFile's mode to force execution
		op.File.Options.Mode = "force"
		
		var err error
		if e.action == "rename" {
			err = actRename(op.File, e.action)
		} else {
			var targetFolder string
			if op.IsMovie {
				targetFolder = e.movieFolder
			} else {
				targetFolder = e.seriesFolder
			}
			err = act(op.File, targetFolder, e.action)
		}
		
		if err != nil {
			log.WithFields(log.Fields{"error": err, "source": op.SourcePath, "target": op.TargetPath}).Errorln("Error processing file")
		}
	}
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

	file := identify.NewParsedFile(filePath, identify.Options{Lookup: e.tmdbLookup, MovieFormat: *movieFormat, SeriesFormat: *seriesFormat, ForceMovie: e.forceMovie, ForceSeries: e.forceSeries, Mode: e.mode})

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

	if p.Options.Mode == "dry-run" {
		log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("DRY-RUN: Would rename file")
		return nil
	}
	
	// For interactive and force modes, we proceed with the action
	if p.Options.Mode == "interactive" {
		// In interactive mode, we still execute since the user will have already confirmed
		// The confirmation happens at a higher level
		log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("Acting on file")
	} else if p.Options.Mode == "force" {
		log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("Acting on file")
	}

	if _, err := os.Lstat(targetLocation); err == nil {
		log.Warnln("File already exists, doing nothing.")
		return nil
	}

	err = os.Rename(source, targetLocation)
	if err != nil {
		return err
	}

	return nil
}

func act(p identify.ParsedFile, targetFolder, action string) error {
	source, err := filepath.Abs(p.SourcePath())
	if err != nil {
		return err
	}

	targetLocation := filepath.Join(targetFolder, p.TargetName())

	if p.Options.Mode == "dry-run" {
		log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("DRY-RUN: Would act on file")
		return nil
	}

	// For interactive and force modes, we proceed with the action
	err = ensurePath(filepath.Dir(targetLocation))
	if err != nil {
		return err
	}

	if p.Options.Mode == "interactive" {
		// In interactive mode, we still execute since the user will have already confirmed
		// The confirmation happens at a higher level
		log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("Acting on file")
	} else if p.Options.Mode == "force" {
		log.WithFields(log.Fields{"target": targetLocation, "source": source, "action": action}).Infoln("Acting on file")
	}

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

	return nil
}

// StartRun starts a identification run
func (e *App) StartRun(path string) {
	// Handle interactive mode specially
	if e.mode == "interactive" {
		operations, err := e.collectPlannedOperations(path)
		if err != nil {
			log.WithFields(log.Fields{"path": path, "error": err}).Errorf("could not collect planned operations")
			return
		}
		
		if e.promptForConfirmation(operations) {
			fmt.Println("\nProceeding with operations...")
			e.executeOperations(operations)
			fmt.Printf("Completed processing %d file(s).\n", len(operations))
		} else {
			fmt.Println("Operation cancelled by user.")
		}
		return
	}
	
	// Handle dry-run and force modes with existing logic
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
