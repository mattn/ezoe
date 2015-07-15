package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bgentry/speakeasy"
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
	you    = flag.String("u", "EzoeRyou", "who do you want to ask to?")
	stdout = colorable.NewColorableStdout()
)

var highlights = []*regexp.Regexp{
	regexp.MustCompile("質問ではない。?"),
	regexp.MustCompile("不?自由"),
}

func doLogin() error {
	uri, err := url.Parse("http://ask.fm/")
	if err != nil {
		return err
	}

	me, err := user.Current()
	if err != nil {
		return err
	}
	session := filepath.Join(me.HomeDir, ".ezoe.session")
	if b, err := ioutil.ReadFile(session); err == nil {
		http.DefaultClient.Jar.SetCookies(uri, []*http.Cookie{&http.Cookie{
			Name:  "_ask.fm_session",
			Value: strings.TrimSpace(string(b)),
		}})
	}

	doc, err := goquery.NewDocument("http://ask.fm/login")
	if err != nil {
		return err
	}

	if doc.Find(".link-logout").Length() > 0 {
		return nil
	}

	token := ""
	doc.Find("input[name=authenticity_token]").Each(func(_ int, s *goquery.Selection) {
		if attr, ok := s.Attr("value"); ok {
			token = attr
		}
	})

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("User: ")
		if scanner.Scan() {
			break
		}
	}
	user := scanner.Text()
	password, err := speakeasy.Ask("Password: ")
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Add("authenticity_token", token)
	params.Add("login", user)
	params.Add("password", password)

	req, err := http.NewRequest("POST", "http://ask.fm/session", strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Referer", "http://ask.fm/login")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}

	doc, err = goquery.NewDocument("http://ask.fm/account/wall")
	if err != nil {
		return err
	}

	if doc.Find(".link-logout").Length() == 0 {
		return errors.New("正しいユーザもしくはパスワードではない")
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "_ask.fm_session" {
			return ioutil.WriteFile(session, []byte(cookie.Value), 0600)
		}
	}
	return nil
}

func doPost(question string) error {
	doc, err := goquery.NewDocument("http://ask.fm/" + *you)
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

	req, err := http.NewRequest("POST", "http://ask.fm/"+*you+"/questions/create", strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Referer", "http://ask.fm/"+*you)
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

func doList() error {
	return feeder.New(20, true, nil,
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
					desc = h.ReplaceAllString(desc, Green+"$0"+White)
				}
				fmt.Fprintln(stdout, "  "+White+desc+End)
				fmt.Println("")
			}
		},
	).Fetch("http://ask.fm/feed/profile/"+*you+".rss", nil)
}

func doEzoe() error {
	flag.Parse()

	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	http.DefaultClient.Jar = jar

	if flag.NArg() > 0 {
		if err = doLogin(); err != nil {
			return err
		}
		return doPost(strings.Join(flag.Args(), " "))
	}
	return doList()
}

func main() {
	if err := doEzoe(); err != nil {
		fmt.Fprintln(os.Stderr, os.Args[0]+":", err)
		os.Exit(1)
	}
}
