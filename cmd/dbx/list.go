package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jnsgruk/dbx/internal/lxc"
	"github.com/jnsgruk/dbx/internal/state"
	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tracked instances",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st := state.LoadPruned()

			if len(st) == 0 {
				fmt.Println("No tracked instances.")
				return nil
			}

			allInfo, err := lxc.ListInstanceInfo()
			if err != nil {
				return fmt.Errorf("listing instances: %w", err)
			}

			type entry struct {
				name string
				dir  string
				info lxc.InstanceInfo
			}

			home, _ := os.UserHomeDir()
			entries := make([]entry, 0, len(st))
			for dir, names := range st {
				display := dir
				if home != "" && strings.HasPrefix(dir, home) {
					display = "~" + dir[len(home):]
				}
				for _, name := range names {
					entries = append(entries, entry{name: name, dir: display, info: allInfo[name]})
				}
			}
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].info.CreatedAt.After(entries[j].info.CreatedAt)
			})

			headers := [5]string{"NAME", "STATUS", "IPV4", "DIRECTORY", "CREATED"}
			widths := [5]int{len(headers[0]), len(headers[1]), len(headers[2]), len(headers[3]), len(headers[4])}

			type row [5]string
			rows := make([]row, len(entries))
			for i, e := range entries {
				rows[i] = row{e.name, e.info.Status, e.info.IPv4, e.dir, timeAgo(e.info.CreatedAt)}
				for c := range widths {
					if len(rows[i][c]) > widths[c] {
						widths[c] = len(rows[i][c])
					}
				}
			}

			fmtStr := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%s\n",
				widths[0], widths[1], widths[2], widths[3])
			fmt.Printf(fmtStr, headers[0], headers[1], headers[2], headers[3], headers[4])
			for _, r := range rows {
				fmt.Printf(fmtStr, r[0], r[1], r[2], r[3], r[4])
			}
			return nil
		},
	}
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		days := int(math.Round(d.Hours() / 24))
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 30*24*time.Hour:
		weeks := int(math.Round(d.Hours() / (24 * 7)))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		months := int(math.Round(d.Hours() / (24 * 30)))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}
