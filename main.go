package main

import (
	"flag"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"os"
	"strings"
	"time"
)

func main() {

	pathFlag := flag.String("path", "", "The path to the repository to be analyzed")
	yearFlag := flag.Int("year", 2023, "The year for which the wrapped should be generated. Default=2023")
	emailsFlag := flag.String("emails", "", "A comma separated list of emails to identify the author")
	flag.Parse()

	if *pathFlag == "" {
		fmt.Printf("Forgot to specify the --path to the git repository")
		flag.Usage()
		os.Exit(1)
	}

	if *emailsFlag == "" {
		fmt.Printf("Forgot to specify a valid email address of the author for which the wrapped will be created")
		flag.Usage()
		os.Exit(1)
	}

	emails := make(map[string]bool)
	splitEmailFlag := strings.Split(*emailsFlag, ",")
	for _, email := range splitEmailFlag {
		emails[strings.TrimSpace(email)] = true
	}

	if len(emails) == 0 {
		fmt.Printf("No valid author emails were provided")
		flag.Usage()
		os.Exit(1)
	}

	err := getWrapped(*pathFlag, *yearFlag, emails)
	if err != nil {
		fmt.Printf("Error generating your wrapped. [err=%s]\n", err.Error())
		os.Exit(1)
	}
}

func getWrapped(path string, year int, authors map[string]bool) error {

	repo, err := git.PlainOpen(path)
	if err != nil {
		return err
	}

	commits, err := findRelevantCommits(repo, year, authors)
	if err != nil {
		return err
	}

	if len(commits) == 0 {
		return fmt.Errorf("unable to generate a git-wrapped for the provided author, no commits were found!")
	}

	summary, err := analyze(commits)
	if err != nil {
		return err
	}

	output := buildOutput(summary)
	fmt.Println(output)

	return err
}

func findRelevantCommits(repo *git.Repository, year int, authors map[string]bool) ([]*object.Commit, error) {
	startTime := time.Date(year, 1, 1, 0, 1, 0, 0, time.Local)
	endTime := time.Date(year, 12, 31, 0, 1, 0, 0, time.Local)

	commits, err := repo.CommitObjects()
	if err != nil {
		return nil, err
	}
	defer commits.Close()

	authoredCommits := make([]*object.Commit, 0)
	err = commits.ForEach(func(commit *object.Commit) error {
		authorSig := commit.Author
		if authorSig.When.After(startTime) && authorSig.When.Before(endTime) {
			if _, ok := authors[authorSig.Email]; ok {
				authoredCommits = append(authoredCommits, commit)
			}
		}

		return nil
	})

	return authoredCommits, nil
}

type wrappedSummary struct {
	TotalCommits     int64
	Earliest         *object.Commit
	Latest           *object.Commit
	Largest          *object.Commit
	Smallest         *object.Commit
	AverageAdditions int64
	AverageDeletions int64
	ByDay            map[int][]*object.Commit
}

func timeToInt(t time.Time) int {
	return t.Hour()*10000 + t.Minute()*100 + t.Second()
}

func analyze(commits []*object.Commit) (*wrappedSummary, error) {

	summary := &wrappedSummary{
		TotalCommits: int64(len(commits)),
		Earliest:     commits[0],
		Latest:       commits[0],
		ByDay:        make(map[int][]*object.Commit),
	}
	earliestTime := timeToInt(summary.Earliest.Author.When)
	latestTime := timeToInt(summary.Latest.Author.When)
	additionCount := int64(0)
	deletionCount := int64(0)

	for _, commit := range commits {

		whenInt := timeToInt(commit.Author.When)
		// Earliest
		if whenInt < earliestTime {
			earliestTime = whenInt
			summary.Earliest = commit
		}

		// Latest
		if whenInt > latestTime {
			latestTime = whenInt
			summary.Latest = commit
		}

		stats, err := commit.Stats()
		if err != nil {
			return nil, err
		}
		for _, stat := range stats {
			additionCount += int64(stat.Addition)
			deletionCount += int64(stat.Deletion)
		}

		// ByDay
		if byDay, ok := summary.ByDay[commit.Author.When.YearDay()]; ok {
			summary.ByDay[commit.Author.When.YearDay()] = append(byDay, commit)
		} else {
			byDay := make([]*object.Commit, 1)
			byDay[0] = commit
			summary.ByDay[commit.Author.When.YearDay()] = byDay
		}
	}

	summary.AverageAdditions = additionCount / int64(len(commits))
	summary.AverageDeletions = deletionCount / int64(len(commits))

	return summary, nil
}

func buildOutput(summary *wrappedSummary) string {
	var mostDay []*object.Commit

	for _, byDay := range summary.ByDay {
		if len(byDay) > len(mostDay) {
			mostDay = byDay
		}
	}

	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf("🧮 Total commit count: %d\n", summary.TotalCommits))
	builder.WriteString(fmt.Sprintf("🌅 Earliest commit(%v): %s -- %s\n", summary.Earliest.Author.When, summary.Earliest.Hash.String(), strings.TrimSpace(summary.Earliest.Message)))
	builder.WriteString(fmt.Sprintf("🌃 Latest commit(%v): %s -- %s\n", summary.Latest.Author.When, summary.Latest.Hash.String(), strings.TrimSpace(summary.Latest.Message)))
	builder.WriteString(fmt.Sprintf("🟢 Average addition count: %d\n", summary.AverageAdditions))
	builder.WriteString(fmt.Sprintf("🔴 Average deletion count: %d\n", summary.AverageDeletions))
	if len(summary.ByDay) != 0 {
		mostDay[0].Type()
		builder.WriteString(fmt.Sprintf("🏔️ Most commits per day(%v): %d\n", mostDay[0].Author.When, len(mostDay)))
	}

	return builder.String()
}
