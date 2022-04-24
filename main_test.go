package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestExtract(t *testing.T) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "or")
	defer os.RemoveAll(tmpdir)
	if err != nil {
		t.Error(err)
	}
	e := NewApp(true, "symlink", tmpdir, filepath.Join(tmpdir, "extracted"), tmpdir, false, true, false, "0")
	e.StartRun(filepath.Join("test-files", "The.Matrix-1999.mkv.zip"))
	if err != nil {
		t.Error(err)
	}

	target := filepath.Join(tmpdir, "extracted", "The.Matrix-1999.mkv")
	if _, err := os.Lstat(target); err == nil {
		t.Log("Exists")
	} else if os.IsNotExist(err) {
		t.Error(err)
	}
}

func TestSmallFile(t *testing.T) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "bis")
	defer os.RemoveAll(tmpdir)

	fmt.Println(tmpdir)
	e := NewApp(true, "symlink", tmpdir, filepath.Join(tmpdir, "extracted"), tmpdir, false, true, false, "120")
	e.StartRun(filepath.Join("test-files", "The.Matrix-1999.mkv"))
	if err != nil {
		t.Error(err)
	}
	target := filepath.Join(tmpdir, "The Matrix", "The Matrix (1999).mkv")
	if _, err := os.Lstat(target); err == nil {
		t.Error("An error should have been thrown since the minFileSize was not met, it renamed it anyway!")
	}
}
func TestRecursive(t *testing.T) {
	dir := "The.Call.of.the.Wild.2020.1080p.WEB-DL.H264.AC3-EVO"
	file := "The.Call.of.the.Wild.1080p.WEB-DL.DD5.1.H.264-EVO.mkv"
	tmpdir, err := ioutil.TempDir(os.TempDir(), "bis")
	err = os.MkdirAll(filepath.Join(tmpdir, dir), os.ModePerm)
	tf := filepath.Join(tmpdir, dir, file)
	err = createFile(tf)
	t.Log(err)

	f := NewParsedFile(tf)
	err = f.Act(tmpdir, "symlink")
	if err != nil {
		t.Error(err)
	}
	target := filepath.Join(tmpdir, f.TargetName())
	if _, err := os.Lstat(target); err == nil {
		t.Log("Exists")
	} else if os.IsNotExist(err) {
		t.Error(err)
	}
}

func TestSymlink(t *testing.T) {
	name := "Angel.S04E02.mkv"
	tmpdir, err := stageTestFolder(name)
	defer os.RemoveAll(tmpdir)

	if err != nil {
		t.Error(err)
	}

	f := NewParsedFile(filepath.Join(tmpdir, "Angel.S04E02.mkv"))
	err = f.Act(tmpdir, "symlink")
	if err != nil {
		t.Error(err)
	}
	target := filepath.Join(tmpdir, f.TargetName())
	if _, err := os.Lstat(target); err == nil {
		t.Log("Exists")
	} else if os.IsNotExist(err) {
		t.Error(err)
	}
}

func TestCopy(t *testing.T) {
	name := "Angel.S04E02.mkv"
	tmpdir, err := stageTestFolder(name)
	defer os.RemoveAll(tmpdir)

	if err != nil {
		t.Error(err)
	}

	f := NewParsedFile(filepath.Join(tmpdir, name))
	err = f.Act(tmpdir, "copy")
	if err != nil {
		t.Error(err)
	}
	target := filepath.Join(tmpdir, f.TargetName())
	if _, err := os.Stat(target); err == nil {
		t.Log("Exists")
	} else if os.IsNotExist(err) {
		t.Error(err)
	}
}
func createFile(fileName string) error {
	_, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		file, err := os.Create(fileName)
		if err != nil {
			return err
		}
		defer file.Close()
	}
	return nil
}

func stageTestFolder(fileName string) (string, error) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "bis")

	err = createFile(filepath.Join(tmpdir, fileName))
	if err != nil {
		return "", err
	}
	return tmpdir, err
}

func TestMove(t *testing.T) {
	name := "Angel.S04E02.mkv"
	tmpdir, err := stageTestFolder(name)
	defer os.RemoveAll(tmpdir)

	if err != nil {
		t.Error(err)
	}

	f := NewParsedFile(filepath.Join(tmpdir, name))
	err = f.Act(tmpdir, "move")
	if err != nil {
		t.Error(err)
	}
	target := filepath.Join(tmpdir, f.TargetName())
	if _, err := os.Stat(target); err == nil {
		t.Log("Exists")
	} else if os.IsNotExist(err) {
		t.Error(err)
	}
}
func TestLookup(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	moreTests := make(map[string]ParsedFile)
	moreTests["The.Flash.2014.S06E07.720p.HDTV.x264-SVA.mkv"] = ParsedFile{Filename: "The.Flash.2014.S06E07.720p.HDTV.x264-SVA", Extension: ".mkv", Filepath: "The.Flash.2014.S06E07.720p.HDTV.x264-SVA.mkv", Year: "2014", IsMovie: false, IsSeries: true, CleanName: "The Flash (2014)", Season: "06", Episode: "07", Resolution: "720p"}
	moreTests["Charmed.1998.S01E01.mkv"] = ParsedFile{Filename: "Charmed.1998.S01E01", Extension: ".mkv", Filepath: "Charmed.1998.S01E01.mkv", Year: "1998", IsMovie: false, IsSeries: true, CleanName: "Charmed (1998)", Season: "01", Episode: "01", Resolution: ""}
	moreTests["Charmed.2018.S01E01.mkv"] = ParsedFile{Filename: "Charmed.2018.S01E01", Extension: ".mkv", Filepath: "Charmed.2018.S01E01.mkv", Year: "2018", IsMovie: false, IsSeries: true, CleanName: "Charmed (2018)", Season: "01", Episode: "01", Resolution: ""}
	moreTests["Maleficent.Mistress.of.Evil.2019.720p.BluRay.x264-SPARKS.mkv"] = ParsedFile{Filename: "Maleficent.Mistress.of.Evil.2019.720p.BluRay.x264-SPARKS", Extension: ".mkv", Filepath: "Maleficent.Mistress.of.Evil.2019.720p.BluRay.x264-SPARKS.mkv", Year: "2019", IsMovie: true, IsSeries: false, CleanName: "Maleficent Mistress of Evil", Season: "", Episode: "", Resolution: "720p"}
	for name, mi := range moreTests {
		newMi := NewParsedFile(name, Options{Lookup: true})
		if newMi.Extension != mi.Extension {
			t.Errorf("Extension '%v' did not match expected extension '%v'\n", newMi.Extension, mi.Extension)
		}

		if newMi.Filename != mi.Filename {
			t.Errorf("Filename '%v' did not match expected Filename '%v'\n", newMi.Filename, mi.Filename)
		}

		if newMi.Filepath != mi.Filepath {
			t.Errorf("Filepath '%v' did not match expected Filepath '%v'\n", newMi.Filepath, mi.Filepath)
		}

		if newMi.CleanName != mi.CleanName {
			t.Errorf("Auto-parsed CleanName '%v' did not match hardcoded CleanName '%v'\n", newMi.CleanName, mi.CleanName)
		}

		if newMi.Season != mi.Season {
			t.Errorf("Season '%v' did not match expected Season '%v'\n", newMi.Season, mi.Season)
		}

		if newMi.Episode != mi.Episode {
			t.Errorf("Episode '%v' did not match expected Episode '%v'\n", newMi.Episode, mi.Episode)
		}

		if newMi.FullName() != mi.FullName() {
			t.Errorf("FullName '%v' did not match expected FullName '%v'\n", newMi.FullName(), mi.FullName())
		}

		if newMi.Year != mi.Year {
			t.Errorf("Year '%v' did not match expected Year '%v'\n", newMi.Year, mi.Year)
		}

		if newMi.IsMovie != mi.IsMovie {
			t.Errorf("Expected '%v' to be a movie, but it was not. Season: '%s', Episode: '%s'", newMi.Filename, newMi.Season, newMi.Episode)
		}

		if newMi.IsSeries != mi.IsSeries {
			t.Errorf("Expected '%v' to be a series, but it was not.", newMi.Filename)
		}

		if newMi.TargetName() != mi.TargetName() {
			t.Errorf("TargetName() '%v' did not match expected TargetName() '%v'\n", newMi.TargetName(), mi.TargetName())
		}
	}
}
func TestForceContentType(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	f := NewParsedFile("This.Is.A.Series.S01E22.mkv", Options{ForceMovie: true})
	if !f.IsMovie {
		t.Errorf("Expected '%s' to be identified as a movie since we forced it even though it looked like an episode %v", f.CleanName, f.IsMovie)
	}

}

func TestParseContent(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	tests := make(map[string]ParsedFile)
	tests["The Matrix Revolutions (2003).mkv"] = ParsedFile{Filename: "The Matrix Revolutions (2003)", Extension: ".mkv", Filepath: "The Matrix Revolutions (2003).mkv", Year: "2003", IsMovie: true, CleanName: "The Matrix Revolutions"}
	tests["/home/test/The.Call.of.the.Wild.2020.1080p.WEB-DL.H264.AC3-EVO/The.Call.of.the.Wild.1080p.WEB-DL.DD5.1.H.264-EVO.mkv"] = ParsedFile{Filename: "The.Call.of.the.Wild.2020.1080p.WEB-DL.H264.AC3-EVO", Extension: ".mkv", Filepath: "The.Call.of.the.Wild.2020.1080p.WEB-DL.H264.AC3-EVO.mkv", Year: "2020", IsMovie: true, CleanName: "The Call Of The Wild", Resolution: "1080p"}
	tests["Sonic.the.Hedgehog.2020.1080p.HDRip.X264.AC3-EVO.mkv"] = ParsedFile{Filename: "Sonic.the.Hedgehog.2020.1080p.HDRip.X264.AC3-EVO", Extension: ".mkv", Filepath: "Sonic.the.Hedgehog.2020.1080p.HDRip.X264.AC3-EVO.mkv", Year: "2020", IsMovie: true, Resolution: "1080p", CleanName: "Sonic The Hedgehog"}
	tests["home/data/settings/content/The Matrix Revolutions - 2003.mkv"] = ParsedFile{Filename: "The Matrix Revolutions - 2003", Extension: ".mkv", Filepath: "home/data/settings/content/The Matrix Revolutions - 2003.mkv", Year: "2003", IsMovie: true, CleanName: "The Matrix Revolutions"}
	tests["Angel.S04E12.mkv"] = ParsedFile{Filename: "Angel.S04E12", Extension: ".mkv", Filepath: "Angel.S04E12.mkv", Year: "", IsSeries: true, CleanName: "Angel", Season: "04", Episode: "12"}
	tests["Downton Abbey 5x06 HDTV x264-FoV [eztv].mkv"] = ParsedFile{Extension: ".mkv", IsSeries: true, Filename: "Downton Abbey 5x06 HDTV x264-FoV [eztv]", Season: "05", Episode: "06", CleanName: "Downton Abbey", Filepath: "Downton Abbey 5x06 HDTV x264-FoV [eztv].mkv"}
	tests["Weekend.At.Bernie's.1989.1080p.BluRay.FLAC2.0.x264-DON.mkv"] = ParsedFile{Filename: "Weekend.At.Bernie's.1989.1080p.BluRay.FLAC2.0.x264-DON", Extension: ".mkv", Filepath: "Weekend.At.Bernie's.1989.1080p.BluRay.FLAC2.0.x264-DON.mkv", Resolution: "1080p", Year: "1989", IsSeries: false, IsMovie: true, CleanName: "Weekend At Bernie's"}
	tests["[HorribleSubs] Kaiji S2 - Against All Rules - 01 [480p].mkv"] = ParsedFile{Filename: "[HorribleSubs] Kaiji S2 - Against All Rules - 01 [480p]", Extension: ".mkv", Filepath: "[HorribleSubs] Kaiji S2 - Against All Rules - 01 [480p].mkv", Year: "", IsSeries: true, CleanName: "Kaiji S2 - Against All Rules", Season: "00", Episode: "01", Resolution: "480p"}
	tests["[HorribleSubs] Fruits Basket (2019) - 01 [1080p].mkv"] = ParsedFile{Filename: "[HorribleSubs] Fruits Basket (2019) - 01 [1080p]", Extension: ".mkv", Filepath: "[HorribleSubs] Fruits Basket (2019) - 01 [1080p].mkv", Year: "2019", IsSeries: true, CleanName: "Fruits Basket", Season: "00", Episode: "01", Resolution: "1080p"}
	tests["Apollo.11.2019.1080p.mkv"] = ParsedFile{Filename: "Apollo.11.2019.1080p", Extension: ".mkv", Filepath: "Apollo.11.2019.1080p.mkv", Year: "2019", IsMovie: true, IsSeries: false, CleanName: "Apollo 11", Season: "", Episode: "", Resolution: "1080p"}
	tests["The.Flash.2014.S06E07.720p.HDTV.x264-SVA.mkv"] = ParsedFile{Filename: "The.Flash.2014.S06E07.720p.HDTV.x264-SVA", Extension: ".mkv", Filepath: "The.Flash.2014.S06E07.720p.HDTV.x264-SVA.mkv", Year: "2014", IsMovie: false, IsSeries: true, CleanName: "The Flash (2014)", Season: "06", Episode: "07", Resolution: "720p"}
	tests["/home/test/The.Flash.2014.S06E07.720p.HDTV.x264-SVA/jioasdjioasd9012.mkv"] = ParsedFile{Filename: "The.Flash.2014.S06E07.720p.HDTV.x264-SVA", Extension: ".mkv", Filepath: "The.Flash.2014.S06E07.720p.HDTV.x264-SVA.mkv", Year: "2014", IsMovie: false, IsSeries: true, CleanName: "The Flash (2014)", Season: "06", Episode: "07", Resolution: "720p"}
	tests["/home/test/letsnotrecurse.mkv"] = ParsedFile{Filename: "test", Extension: ".mkv", Filepath: "test.mkv", Year: "", IsMovie: false, IsSeries: false, CleanName: "Test", Season: "", Episode: "", Resolution: ""}

	for name, mi := range tests {
		newMi := NewParsedFile(name)
		if newMi.Extension != mi.Extension {
			t.Errorf("Extension '%v' did not match expected extension '%v'\n", newMi.Extension, mi.Extension)
		}

		if newMi.Filename != mi.Filename {
			t.Errorf("Filename '%v' did not match expected Filename '%v'\n", newMi.Filename, mi.Filename)
		}

		if newMi.Filepath != mi.Filepath {
			t.Errorf("Filepath '%v' did not match expected Filepath '%v'\n", newMi.Filepath, mi.Filepath)
		}

		if newMi.CleanName != mi.CleanName {
			t.Errorf("CleanName '%v' did not match expected CleanName '%v'\n", newMi.CleanName, mi.CleanName)
		}

		if newMi.Season != mi.Season {
			t.Errorf("Season '%v' did not match expected Season '%v'\n", newMi.Season, mi.Season)
		}

		if newMi.Episode != mi.Episode {
			t.Errorf("Episode '%v' did not match expected Episode '%v'\n", newMi.Episode, mi.Episode)
		}

		if newMi.FullName() != mi.FullName() {
			t.Errorf("FullName '%v' did not match expected FullName '%v'\n", newMi.FullName(), mi.FullName())
		}

		if newMi.Year != mi.Year {
			t.Errorf("Year '%v' did not match expected Year '%v'\n", newMi.Year, mi.Year)
		}

		if newMi.IsMovie != mi.IsMovie {
			t.Errorf("Expected '%v' to be a movie, but it was not. Season: '%s', Episode: '%s'", newMi.Filename, newMi.Season, newMi.Episode)
		}

		if newMi.IsSeries != mi.IsSeries {
			t.Errorf("Expected '%v' to be a series, but it was not.", newMi.Filename)
		}

		if newMi.TargetName() != mi.TargetName() {
			t.Errorf("TargetName() '%v' did not match expected TargetName() '%v'\n", newMi.TargetName(), mi.TargetName())
		}
	}

}
