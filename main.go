package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"regexp"
	"sort"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

var app = &cli.App{
	Name:  "delete-old-files",
	Usage: "delete given number of oldest files matching given pattern",

	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "pattern",
			Aliases:  []string{"e", "p"},
			Usage:    "regular expression used to match files",
			Required: true,
		},
		&cli.IntFlag{
			Name:    "number",
			Aliases: []string{"n"},
			Usage:   "number of files to delete if positive, or to keep if negative; ALL matching files will be deleted if set to 0",
			Value:   0,
		},
		&cli.StringFlag{
			Name:    "path",
			Aliases: []string{"P", "prefix"},
			Usage:   "path to working directory",
			Value:   ".",
		},
		&cli.BoolFlag{
			Name:    "dry-run",
			Aliases: []string{"no"},
			Usage:   "print files to be deleted without deleting",
			Value:   false,
		},
		&cli.BoolFlag{
			Name:    "yes",
			Aliases: []string{"y"},
			Usage:   "delete files without confirmation",
			Value:   false,
		},
	},

	Action: func(ctx *cli.Context) error {
		pattern := ctx.String("pattern")
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("failed to compile pattern: %w", err)
		}

		fileNameSelf := path.Base(os.Args[0])
		prefix := ctx.String("path")
		arrFileInfo, err := listByTime(prefix)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		arrMatch := make([]os.FileInfo, 0, len(arrFileInfo))
		for _, fi := range arrFileInfo {
			if fi.Name() == fileNameSelf {
				continue
			}
			if re.MatchString(fi.Name()) {
				arrMatch = append(arrMatch, fi)
			}
		}

		if len(arrMatch) == 0 {
			fmt.Println("No matching files found")
			return nil
		}

		number := ctx.Int("number")
		if number > 0 {
			if len(arrMatch) > number {
				arrMatch = arrMatch[:number]
			}
		} else if number < 0 {
			if len(arrMatch) > -number {
				arrMatch = arrMatch[:len(arrMatch)+number]
			} else {
				fmt.Println("All matching files will be kept due to given number flag")
				return nil
			}
		}

		printResult(arrMatch)

		if ctx.Bool("dry-run") {
			return nil
		}

		confirm := false
		if !ctx.Bool("yes") {
			ans := "No"
			survey.AskOne(&survey.Select{
				Message: "All files above will be deleted, continue?",
				Options: []string{"Yes", "No", "Pick"},
				Default: "No",
			}, &ans)
			switch ans {
			case "Yes":
				confirm = true
			case "No":
				confirm = false
			case "Pick":
				{
					mapSel := make(map[string]fs.FileInfo, len(arrMatch))
					arrSel := make([]string, 0, len(arrMatch))
					for _, fi := range arrMatch {
						mapSel[fi.Name()] = fi
						arrSel = append(arrSel, fi.Name())
					}
					arrAns := make([]string, 0, len(arrMatch))
					survey.AskOne(&survey.MultiSelect{
						Message: "Select files to delete",
						Options: arrSel,
						Default: arrSel,
					}, &arrAns)
					arrMatch = make([]os.FileInfo, 0, len(arrAns))
					for _, ans := range arrAns {
						arrMatch = append(arrMatch, mapSel[ans])
					}

					confirm = len(arrMatch) > 0
				}
			}
		} else {
			confirm = true
		}

		if !confirm {
			fmt.Println("Aborted")
			return nil
		}

		hasErr := false
		for _, fi := range arrMatch {
			err = os.Remove(path.Join(prefix, fi.Name()))
			if err != nil {
				log.Println(err)
				hasErr = true
			}
		}

		if hasErr {
			return fmt.Errorf("Failed to delete some files")
		}
		fmt.Println("Finished deleting above files")
		return nil
	},

	HideHelpCommand: true,
}

func listByTime(dir string) ([]os.FileInfo, error) {
	fdDir, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open directory: %w", err)
	}
	defer fdDir.Close()
	if i, _ := fdDir.Stat(); !i.IsDir() {
		return nil, fmt.Errorf("given path is not a directory")
	}

	arrFiles, err := fdDir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("failed to read entries in directory: %w", err)
	}

	arrFileInfo := make([]os.FileInfo, 0, len(arrFiles))

	for _, f := range arrFiles {
		if f.IsDir() {
			continue
		}
		if fi, err := f.Info(); err == nil {
			arrFileInfo = append(arrFileInfo, fi)
		}
	}

	sort.Sort(sortByModtime(arrFileInfo))
	return arrFileInfo, nil
}

func printResult(r []os.FileInfo) {
	const truncateLen = 30

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"FileName", "Size", "ModTime"})
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoFormatHeaders(false)

	table.SetBorder(false)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)

	var totalSize uint64

	for i, fi := range r {
		size := uint64(fi.Size())
		totalSize += size
		if i >= truncateLen {
			continue
		}
		table.Append([]string{
			fi.Name(),
			humanize.IBytes(size),
			fi.ModTime().Format("2006-01-02 15:04:05")})
	}
	table.SetAlignment(tablewriter.ALIGN_RIGHT)

	hint := fmt.Sprintf("Total: %d file(s), %s", len(r), humanize.IBytes(totalSize))
	if len(r) > truncateLen {
		hint = fmt.Sprintf("Showing first %d files only, %s", truncateLen, hint)
	}

	table.Render()
	fmt.Println(hint)
}

func main() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type sortByModtime []os.FileInfo

func (s sortByModtime) Len() int {
	return len(s)
}
func (s sortByModtime) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s sortByModtime) Less(i, j int) bool {
	return s[i].ModTime().Before(s[j].ModTime())
}
