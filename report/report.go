package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
)

// DataPoint represents one of the comparison points.
type DataPoint struct {
	Name       string
	Commit     string
	Size       int64
	Minified   int64
	Compressed int64
}

// delta returns string describing size change between two sizes.
func delta(proposed, baseline int64) string {
	pct := float32(proposed-baseline) / float32(baseline) * 100

	keyword := "increase"
	if pct < 0 {
		pct = -pct
		keyword = "decrease"
	}

	return fmt.Sprintf("%+0.2f%% %s (%s bytes)", pct, keyword, humanize.Comma(baseline))
}

// Report contains information about measurements performed.
type Report struct {
	App struct {
		Name   string
		Repo   string
		Commit string
	}
	Trigger      string // Pull request or commit URL.
	Measurements []*DataPoint
}

// String renders a human-readable representation of the report in Markdown format.
func (r Report) String() string {
	if len(r.Measurements) == 0 {
		return "No measurements to report."
	}

	result := strings.Builder{}

	result.WriteString(fmt.Sprintf("Reference app: [%s](%s) (`%s`)\n\n", r.App.Name, r.App.Repo, r.App.Commit))

	table := tablewriter.NewWriter(&result)
	table.SetHeader([]string{"Branch", "Original", "Minified", "Compressed (gzip)"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.SetAutoWrapText(false)

	proposed := r.Measurements[0]
	table.Append([]string{
		proposed.Name,
		humanize.Comma(proposed.Size) + " bytes",
		humanize.Comma(proposed.Minified) + " bytes",
		humanize.Comma(proposed.Compressed) + " bytes",
	})
	for _, baseline := range r.Measurements[1:] {
		table.Append([]string{
			baseline.Name,
			delta(proposed.Size, baseline.Size),
			delta(proposed.Minified, baseline.Minified),
			delta(proposed.Compressed, baseline.Compressed),
		})
	}
	table.Render()

	return result.String()
}

// SaveJSON writes report in JSON format into the given file.
func (r *Report) SaveJSON(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(r)
}

// SaveMarkdown writes report in Markdown format into the given file.
func (r *Report) SaveMarkdown(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(r.String())
	if err != nil {
		return fmt.Errorf("failed to write report table: %w", err)
	}

	return nil
}
