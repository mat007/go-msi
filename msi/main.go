package msi

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/bmatcuk/doublestar"
	"github.com/mat007/go-msi/manifest"
	"github.com/mat007/go-msi/rtf"
	"github.com/mat007/go-msi/templates"
	"github.com/mat007/go-msi/util"
	"github.com/mat007/go-msi/wix"
	"github.com/mh-cbon/stringexec"
	"github.com/urfave/cli"
	"golang.org/x/tools/go/packages"
)

// Version holds the program version.
var Version = "0.0.0"

// TPLPATH points to the template directory on the target system.
// Should be used only for non windows systems to indicate template locations.
var TPLPATH = "" // non-windows build, use ldflags to tell about that.

// Main exposes the application entry point.
func Main() {
	sourceTemplatesDir := func() string {
		meta, _ := packages.Load(
			&packages.Config{Mode: packages.LoadFiles},
			"github.com/mat007/go-msi",
		)
		if len(meta) == 0 {
			return ""
		}
		if len(meta[0].GoFiles) == 0 {
			return ""
		}
		base := filepath.Dir(meta[0].GoFiles[0])
		if _, err := os.Stat(filepath.Join(base, "templates")); err != nil {
			return ""
		}

		return base
	}

	if TPLPATH == "" {
		TPLPATH = sourceTemplatesDir()
	}

	if TPLPATH == "" { // built for windows
		b, err := util.GetBinPath()
		if err != nil {
			panic(err)
		}
		TPLPATH = b
	}
	tmpBuildDir, err := ioutil.TempDir("", "go-msi")
	if err != nil {
		panic(err)
	}

	app := cli.NewApp()
	app.Name = "go-msi"
	app.Version = Version
	app.Usage = "Easy msi pakage for Go"
	app.UsageText = "go-msi <cmd> <options>"
	app.Commands = []cli.Command{
		{
			Name:   "check-env",
			Usage:  "Provide a report about your environment setup",
			Action: checkEnv,
		},
		{
			Name:   "add-files",
			Usage:  "Adds files from your wix manifest",
			Action: addFiles,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "path, p",
					Value: "wix.json",
					Usage: "Path to the wix manifest file",
				},
				cli.StringFlag{
					Name:  "dir",
					Usage: "Base directory from which to include files",
				},
				cli.StringSliceFlag{
					Name:  "includes, i",
					Usage: "Comma separated list of files to include, use of * and ** is permitted",
				},
				cli.StringSliceFlag{
					Name:  "excludes, e",
					Usage: "Comma separated list of files to exclude, use of * and ** is permitted",
				},
				cli.BoolFlag{
					Name:  "test, t",
					Usage: "Test mode, does not modify the wix manifest file but exits with an error instead",
				},
			},
		},
		{
			Name:   "set-guid",
			Usage:  "Sets appropriate guids in your wix manifest",
			Action: setGUID,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "path, p",
					Value: "wix.json",
					Usage: "Path to the wix manifest file",
				},
				cli.BoolFlag{
					Name:  "force, f",
					Usage: "Force update the guids",
				},
			},
		},
		{
			Name:   "generate-templates",
			Usage:  "Generate wix templates",
			Action: generateTemplates,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "path, p",
					Value: "wix.json",
					Usage: "Path to the wix manifest file",
				},
				cli.StringFlag{
					Name:  "src, s",
					Value: filepath.Join(TPLPATH, "templates"),
					Usage: "Directory path to the wix templates files",
				},
				cli.StringFlag{
					Name:  "out, o",
					Value: tmpBuildDir,
					Usage: "Directory path to the generated wix templates files",
				},
				cli.StringFlag{
					Name:  "version",
					Usage: "The version of your program",
				},
				cli.StringFlag{
					Name:  "display",
					Usage: "The display version of your program",
				},
				cli.StringFlag{
					Name:  "license, l",
					Usage: "Path to the license file",
				},
				cli.StringSliceFlag{
					Name:  "property, pr",
					Usage: "A property to set defined as Id=Value",
				},
			},
		},
		{
			Name:   "to-windows",
			Usage:  "Write Windows1252 encoded file",
			Action: toWindows1252,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "src, s",
					Usage: "Path to an UTF-8 encoded file",
				},
				cli.StringFlag{
					Name:  "out, o",
					Usage: "Path to the ANSI generated file",
				},
			},
		},
		{
			Name:   "to-rtf",
			Usage:  "Write RTF formatted file",
			Action: toRtf,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "src, s",
					Usage: "Path to a text file",
				},
				cli.StringFlag{
					Name:  "out, o",
					Usage: "Path to the RTF generated file",
				},
				cli.BoolFlag{
					Name:  "reencode, e",
					Usage: "Also re encode UTF-8 to Windows1252 charset",
				},
			},
		},
		{
			Name:   "gen-wix-cmd",
			Usage:  "Generate a batch file of Wix commands to run",
			Action: generateWixCommands,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "bin, b",
					Usage: "Path to the wix binaries (if not in PATH)",
				},
				cli.StringFlag{
					Name:  "path, p",
					Value: "wix.json",
					Usage: "Path to the wix manifest file",
				},
				cli.StringFlag{
					Name:  "src, s",
					Value: filepath.Join(TPLPATH, "templates"),
					Usage: "Directory path to the wix templates files",
				},
				cli.StringFlag{
					Name:  "out, o",
					Value: tmpBuildDir,
					Usage: "Directory path to the generated wix cmd file",
				},
				cli.StringFlag{
					Name:  "arch, a",
					Usage: "A target architecture, amd64 or 386 (ia64 is not handled)",
				},
				cli.StringFlag{
					Name:  "msi, m",
					Usage: "Path to write resulting msi file to",
				},
			},
		},
		{
			Name:   "run-wix-cmd",
			Usage:  "Run the batch file of Wix commands",
			Action: runWixCommands,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "out, o",
					Value: tmpBuildDir,
					Usage: "Directory path to the generated wix cmd file",
				},
			},
		},
		{
			Name:   "make",
			Usage:  "All-in-one command to make MSI files",
			Action: quickMake,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "bin, b",
					Usage: "Path to the wix binaries (if not in PATH)",
				},
				cli.StringFlag{
					Name:  "path, p",
					Value: "wix.json",
					Usage: "Path to the wix manifest file",
				},
				cli.StringFlag{
					Name:  "src, s",
					Value: filepath.Join(TPLPATH, "templates"),
					Usage: "Directory path to the wix templates files",
				},
				cli.StringFlag{
					Name:  "out, o",
					Value: tmpBuildDir,
					Usage: "Directory path to the generated wix cmd file",
				},
				cli.StringFlag{
					Name:  "arch, a",
					Usage: "A target architecture, amd64 or 386 (ia64 is not handled)",
				},
				cli.StringFlag{
					Name:  "msi, m",
					Usage: "Path to write resulting msi file to",
				},
				cli.StringFlag{
					Name:  "version",
					Usage: "The version of your program",
				},
				cli.StringFlag{
					Name:  "display",
					Usage: "The display version of your program",
				},
				cli.StringFlag{
					Name:  "license, l",
					Usage: "Path to the license file",
				},
				cli.StringSliceFlag{
					Name:  "property, pr",
					Usage: "A property to set defined as Id=Value",
				},
				cli.BoolFlag{
					Name:  "keep, k",
					Usage: "Keep output directory containing build files (useful for debug)",
				},
			},
		},
		{
			Name:   "choco",
			Usage:  "Generate a chocolatey package of your msi files",
			Action: chocoMake,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "path, p",
					Value: "wix.json",
					Usage: "Path to the wix manifest file",
				},
				cli.StringFlag{
					Name:  "src, s",
					Value: filepath.Join(TPLPATH, "templates", "choco"),
					Usage: "Directory path to the wix templates files",
				},
				cli.StringFlag{
					Name:  "version",
					Usage: "The version of your program",
				},
				cli.StringFlag{
					Name:  "out, o",
					Value: tmpBuildDir,
					Usage: "Directory path to the generated chocolatey build file",
				},
				cli.StringFlag{
					Name:  "input, i",
					Usage: "Path to the msi file to package into the chocolatey package",
				},
				cli.StringFlag{
					Name:  "changelog-cmd, c",
					Usage: "A command to generate the content of the changlog in the package",
				},
				cli.BoolFlag{
					Name:  "keep, k",
					Usage: "Keep output directory containing build files (useful for debug)",
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		if e, ok := err.(*cli.ExitError); ok {
			os.Exit(e.ExitCode())
		}
		os.Exit(1)
	}
}

var verReg = regexp.MustCompile(`\s[0-9]+[.][0-9]+[.][0-9]+`)

func checkEnv(c *cli.Context) error {
	for _, b := range []string{"light", "candle"} {
		if out, err := util.Exec(b, "-h"); out == "" {
			fmt.Printf("!!	%v not found: %q\n", b, err)
		} else {
			match := verReg.FindAllString(out, -1)
			if len(match) < 1 {
				fmt.Printf("??	%v probably not found\n", b)
			} else {
				version := strings.TrimSpace(match[0])
				ver, err := semver.NewVersion(version)
				if err != nil {
					fmt.Printf("??	%v found but its version is not parsable %v\n", b, version)
				} else {
					min := "3.10.0"
					if !ver.GreaterThan(semver.MustParse(min)) {
						fmt.Printf("!!	%v found %v but %v is required\n", b, version, min)
					} else {
						fmt.Printf("ok	%v found %v\n", b, version)
					}
				}
			}
		}
	}
	if out, err := util.Exec("choco", "-v"); out == "" {
		fmt.Printf("!!	%v not found: %q\n", "chocolatey", err)
	} else {
		match := verReg.FindAllString(" "+out, -1)
		if len(match) < 1 {
			fmt.Printf("??	%v probably not found\n", "chocolatey")
		} else {
			version := strings.TrimSpace(match[0])
			ver, err := semver.NewVersion(version)
			if err != nil {
				fmt.Printf("??	%v found but its version is not parsable %v\n", "chocolatey", version)
			} else {
				min := "0.10.0"
				if !ver.GreaterThan(semver.MustParse(min)) {
					fmt.Printf("!!	%v found %v but >%v is required\n", "chocolatey", version, min)
				} else {
					fmt.Printf("ok	%v found %v\n", "chocolatey", version)
				}
			}
		}
	}
	return nil
}

func addFiles(c *cli.Context) error {
	path := c.String("path")
	dir := c.String("dir")
	includes := c.StringSlice("includes")
	excludes := c.StringSlice("excludes")
	test := c.Bool("test")

	if dir == "" {
		return cli.NewExitError(fmt.Errorf("--dir argument is required"), 1)
	}
	if len(includes) == 0 {
		return cli.NewExitError(fmt.Errorf("--includes argument is required"), 1)
	}
	wixFile := manifest.WixManifest{}
	err := wixFile.Load(path)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	out := make(map[string]bool)
	err = glob(dir, excludes, func(match string) {
		out[match] = true
	}, false)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	err = glob(dir, includes, func(match string) {
		file := manifest.File{Path: filepath.ToSlash(filepath.Join(dir, match))}
		if out[match] {
			fmt.Println("    excluding", file.Path)
			return
		}
		path := strings.Split(match, "/")
		wixFile.Directory = addFile(wixFile.Directory, file, path)
	}, true)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if test {
		return fmt.Errorf("file list not up to date")
	}
	err = wixFile.Write(path)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	fmt.Println("The file is saved on disk")
	return nil
}

func addFile(dir manifest.Directory, file manifest.File, path []string) manifest.Directory {
	if len(path) == 1 {
		for _, f := range dir.Files {
			if f.Path == file.Path {
				fmt.Println("    skipping", file.Path, "already listed")
				return dir
			}
		}
		fmt.Println("    adding", file.Path)
		dir.Files = append(dir.Files, file)
		return dir
	}
	for i, d := range dir.Directories {
		if d.Name == path[0] {
			dir.Directories[i] = addFile(d, file, path[1:])
			return dir
		}
	}
	d := addFile(manifest.Directory{Name: path[0]}, file, path[1:])
	dir.Directories = append(dir.Directories, d)
	return dir
}

func glob(dir string, patterns []string, f func(match string), fail bool) error {
	for _, pattern := range patterns {
		var matches []string
		for _, file := range strings.Split(pattern, ",") {
			match, err := doublestar.Glob(filepath.Join(dir, file))
			if err != nil {
				return err
			}
			if fail && match == nil {
				return fmt.Errorf("files %q do not exist", pattern)
			}
			matches = append(matches, match...)
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				return err
			}
			if info.IsDir() {
				continue
			}
			rel, err := filepath.Rel(dir, match)
			if err != nil {
				return err
			}
			f(filepath.ToSlash(rel))
		}
	}
	return nil
}

func setGUID(c *cli.Context) error {
	path := c.String("path")
	force := c.Bool("force")

	wixFile := manifest.WixManifest{}
	err := wixFile.Load(path)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	updated, err := wixFile.SetGuids(force)
	if err != nil {
		return err
	}
	if updated {
		fmt.Println("The manifest was updated")
	} else {
		fmt.Println("The manifest was not updated")
	}

	err = wixFile.Write(path)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	fmt.Println("The file is saved on disk")

	return nil
}

func generateTemplates(c *cli.Context) error {
	path := c.String("path")
	src := c.String("src")
	out := c.String("out")
	compression := c.String("compression")
	version := c.String("version")
	display := c.String("display")
	license := c.String("license")
	properties := c.StringSlice("property")

	wixFile := manifest.WixManifest{}
	err := wixFile.Load(path)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if wixFile.NeedGUID() {
		fmt.Println("The manifest needs Guid")
		fmt.Println("To update your file automatically run:")
		fmt.Println("     go-msi set-guid")
		return cli.NewExitError("Cannot proceed, manifest file is incomplete", 1)
	}

	wixFile.Compression = compression
	wixFile.Version.User = version
	wixFile.Version.Display = display

	if c.IsSet("license") {
		wixFile.License = license
	}

	if err := addProperties(&wixFile, properties); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if err := wixFile.Normalize(); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if err := wixFile.RewriteFilePaths(out); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	tpls, err := templates.Find(src, "*.wxs")
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if len(tpls) == 0 {
		return cli.NewExitError("No templates *.wxs found in this directory", 1)
	}

	err = os.MkdirAll(out, 0744)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	for _, tpl := range tpls {
		dst := filepath.Join(out, filepath.Base(tpl))
		err = templates.GenerateTemplate(&wixFile, tpl, dst)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	}

	fmt.Printf("Generated %d templates\n", len(tpls))
	for _, tpl := range tpls {
		dst := filepath.Join(out, filepath.Base(tpl))
		fmt.Printf("- %s\n", dst)
	}

	return nil
}

func toWindows1252(c *cli.Context) error {
	src := c.String("src")
	out := c.String("out")

	if src == "" {
		return cli.NewExitError("--src argument is required", 1)
	}
	if out == "" {
		return cli.NewExitError("--out argument is required", 1)
	}
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return cli.NewExitError(err.Error(), 1)
	}
	os.MkdirAll(filepath.Dir(out), 0744)
	err := rtf.WriteAsWindows1252(src, out)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	return nil
}

func toRtf(c *cli.Context) error {
	src := c.String("src")
	out := c.String("out")
	reencode := c.Bool("reencode")

	if src == "" {
		return cli.NewExitError("--src argument is required", 1)
	}
	if out == "" {
		return cli.NewExitError("--out argument is required", 1)
	}
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return cli.NewExitError(err.Error(), 1)
	}

	os.MkdirAll(filepath.Dir(out), 0744)

	err := rtf.WriteAsRtf(src, out, reencode)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	return nil
}

func generateWixCommands(c *cli.Context) error {
	path := c.String("path")
	src := c.String("src")
	out := c.String("out")
	msi := c.String("msi")
	arch := c.String("arch")
	bin := c.String("bin")

	if msi == "" {
		return cli.NewExitError("--msi parameter must be set", 1)
	}

	templates, err := templates.Find(src, "*.wxs")
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if len(templates) == 0 {
		return cli.NewExitError("No templates *.wxs found in this directory", 1)
	}

	builtTemplates := make([]string, len(templates))
	for i, tpl := range templates {
		builtTemplates[i] = filepath.Join(out, filepath.Base(tpl))
	}

	wixFile := manifest.WixManifest{}
	err = wixFile.Load(path)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if wixFile.NeedGUID() {
		fmt.Println("The manifest needs Guid")
		fmt.Println("To update your file automatically run:")
		fmt.Println("     go-msi set-guid")
		return cli.NewExitError("Cannot proceed, manifest file is incomplete", 1)
	}

	if err := wixFile.Normalize(); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if err := wixFile.RewriteFilePaths(out); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	msi, err = filepath.Abs(msi)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	msi, err = filepath.Rel(out, msi)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if bin != "" {
		if bin, err = filepath.Abs(bin); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	}

	cmdStr := wix.GenerateCmd(&wixFile, builtTemplates, msi, arch, bin)

	targetFile := filepath.Join(out, "build.bat")
	err = ioutil.WriteFile(targetFile, []byte(cmdStr), 0644)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	return nil
}

func runWixCommands(c *cli.Context) error {
	out := c.String("out")

	oCmd := exec.Command("cmd.exe", "/C", "build.bat")
	oCmd.Dir = out
	oCmd.Stdout = os.Stdout
	oCmd.Stderr = os.Stderr
	err := oCmd.Run()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	return nil
}

func quickMake(c *cli.Context) error {
	path := c.String("path")
	src := c.String("src")
	out := c.String("out")
	compression := c.String("compression")
	version := c.String("version")
	display := c.String("display")
	license := c.String("license")
	properties := c.StringSlice("property")
	msi := c.String("msi")
	arch := c.String("arch")
	keep := c.Bool("keep")
	bin := c.String("bin")

	if msi == "" {
		return cli.NewExitError("--msi parameter must be set", 1)
	}

	wixFile := manifest.WixManifest{}
	if err := wixFile.Load(path); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if _, err := wixFile.SetGuids(false); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if err := os.RemoveAll(out); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if err := os.MkdirAll(out, 0744); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	wixFile.Compression = compression
	wixFile.Version.User = version
	wixFile.Version.Display = display

	if c.IsSet("license") {
		wixFile.License = license
	}
	if wixFile.License != "" {
		isRtf, err := rtf.IsRtf(wixFile.License)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		if !isRtf {
			fmt.Println("Converting license to RTF")
			target := filepath.Join(out, filepath.Base(wixFile.License)+".rtf")
			if err := rtf.WriteAsRtf(wixFile.License, target, true); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			wixFile.License = target
		}
	}

	if err := addProperties(&wixFile, properties); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if err := wixFile.Normalize(); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if err := wixFile.RewriteFilePaths(out); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	tpls, err := templates.Find(src, "*.wxs")
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if len(tpls) == 0 {
		return cli.NewExitError("No templates *.wxs found in this directory", 1)
	}

	builtTemplates := make([]string, len(tpls))
	for i, tpl := range tpls {
		dst := filepath.Join(out, filepath.Base(tpl))
		err = templates.GenerateTemplate(&wixFile, tpl, dst)
		builtTemplates[i] = dst
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	}

	msi, err = filepath.Abs(msi)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	msi, err = filepath.Rel(out, msi)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if bin != "" {
		if bin, err = filepath.Abs(bin); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	}

	cmdStr := wix.GenerateCmd(&wixFile, builtTemplates, msi, arch, bin)

	targetFile := filepath.Join(out, "build.bat")
	err = ioutil.WriteFile(targetFile, []byte(cmdStr), 0644)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	oCmd := exec.Command("cmd.exe", "/C", "build.bat")
	oCmd.Dir = out
	oCmd.Stdout = os.Stdout
	oCmd.Stderr = os.Stderr
	err = oCmd.Run()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if keep == false {
		err = os.RemoveAll(out)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	} else {
		fmt.Printf("Build files are available in %s\n", out)
	}

	fmt.Println("All Done!!")

	return nil
}

func addProperties(wixFile *manifest.WixManifest, properties []string) error {
	for _, prop := range properties {
		s := strings.SplitN(prop, "=", 2)
		if len(s) < 2 {
			return fmt.Errorf("property definition must be of the form Id=Value")
		}
		v := manifest.Value(s[1])
		wixFile.Properties = append(wixFile.Properties,
			manifest.Property{
				ID:    s[0],
				Value: &v,
			},
		)
	}
	return nil
}

func chocoMake(c *cli.Context) error {
	path := c.String("path")
	src := c.String("src")
	out := c.String("out")
	input := c.String("input")
	compression := c.String("compression")
	version := c.String("version")
	changelogCmd := c.String("changelog-cmd")
	keep := c.Bool("keep")

	wixFile := manifest.WixManifest{}
	if err := wixFile.Load(path); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if err := os.RemoveAll(out); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if err := os.MkdirAll(out, 0744); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	wixFile.Compression = compression
	wixFile.Version.User = version

	if err := wixFile.Normalize(); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	tpls, err := templates.Find(src, "*")
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if len(tpls) == 0 {
		return cli.NewExitError("No templates found in this directory", 1)
	}

	out, err = filepath.Abs(out)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	wixFile.Choco.BuildDir = out
	wixFile.Choco.MsiFile = filepath.Base(input)
	wixFile.Choco.MsiSum, err = util.ComputeSha256(input)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if changelogCmd != "" {
		windows, err := stringexec.Command(changelogCmd)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		windows.Stderr = os.Stderr
		out, err := windows.Output()
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("Failed to execute command to generate the changelog:%q\n%v", changelogCmd, err.Error()), 1)
		}
		sout := string(out)
		souts := strings.Split(sout, "\n")
		if len(souts) > 2 {
			souts = souts[2:] // why ? command line artifacts ? todo: put an explanation here.
		}
		sout = strings.Join(souts, "\n")

		wixFile.Choco.ChangeLog = sout
	}

	if err := util.CopyFile(filepath.Join(wixFile.Choco.BuildDir, wixFile.Choco.MsiFile), input); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	for _, tpl := range tpls {
		dst := filepath.Join(out, filepath.Base(tpl))
		err := templates.GenerateTemplate(&wixFile, tpl, dst)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	}

	bin, err := exec.LookPath("choco")
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	oCmd := exec.Command(bin, "pack")
	oCmd.Dir = out
	oCmd.Stdout = os.Stdout
	oCmd.Stderr = os.Stderr
	err = oCmd.Run()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	SrcNupkg := fmt.Sprintf("%s\\%s.%s.nupkg", out, wixFile.Choco.ID, wixFile.Version.MSI)
	DstNupkg := fmt.Sprintf("%s.%s.nupkg", wixFile.Choco.ID, wixFile.Version.User)

	if err = util.CopyFile(DstNupkg, SrcNupkg); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if keep == false {
		err = os.RemoveAll(out)
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	} else {
		fmt.Printf("Build files are available in %s\n", out)
	}

	fmt.Printf("Package copied to %s\n", DstNupkg)
	fmt.Println("All Done!!")

	return nil
}
