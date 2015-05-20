package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jteeuwen/go-pkg-rss"
	"github.com/mattn/go-colorable"
)

const (
	Red   = "\x1b[31;1m"
	Green = "\x1b[32;1m"
	White = "\x1b[37;1m"
	End   = "\x1b[0m"
)

var (
	user   = flag.String("u", "EzoeRyou", "who do you want to ask to?")
	stdout = colorable.NewColorableStdout()
)

var highlights = []string{"質問ではない", "自由"}

func post(question string) error {
	doc, err := goquery.NewDocument("http://ask.fm/" + *user)
	if err != nil {
		return err
	}

	token := ""
	doc.Find("input[name=authenticity_token]").Each(func(_ int, s *goquery.Selection) {
		if attr, ok := s.Attr("value"); ok {
			token = attr
		}
	})

	params := url.Values{}
	params.Add("authenticity_token", token)
	params.Add("question[question_text]", question)
	params.Add("question[force_anonymous]", "force_anonymous")

	req, err := http.NewRequest("POST", "http://ask.fm/"+*user+"/questions/create", strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Referer", "http://ask.fm/"+*user)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return nil
}

func main() {
	flag.Parse()

	if flag.NArg() > 0 {
		if err := post(strings.Join(flag.Args(), " ")); err != nil {
			fmt.Fprintln(os.Stderr, os.Args[0]+":", err)
			os.Exit(1)
		}
	} else {
		if err := feeder.New(20, true, nil,
			func(feed *feeder.Feed, ch *feeder.Channel, fi []*feeder.Item) {
			sort_items:
				for n := 0; n < len(fi)-1; n++ {
					tn, err := time.Parse(time.RFC1123Z, fi[n].PubDate)
					if err != nil {
						break sort_items
					}
					for m := len(fi) - 1; m > n; m-- {
						tm, err := time.Parse(time.RFC1123Z, fi[n].PubDate)
						if err != nil {
							break sort_items
						}
						if tn.After(tm) {
							fi[n], fi[m] = fi[m], fi[n]
						}
					}
				}
				for _, item := range fi {
					if len(item.Links) == 0 {
						continue
					}
					// URL
					fmt.Println(item.Links[0].Href)
					// Title
					fmt.Fprintln(stdout, "  "+Red+item.Title+End)
					// Description
					desc := item.Description
					for _, h := range highlights {
						desc = strings.Replace(desc, h, Green+h+White, -1)
					}
					fmt.Fprintln(stdout, "  "+White+desc+End)
					fmt.Println("")
				}
			},
		).Fetch("http://ask.fm/feed/profile/"+*user+".rss", nil); err != nil {
			fmt.Fprintln(os.Stderr, os.Args[0]+":", err)
			os.Exit(1)
		}
	}
}
