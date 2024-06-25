package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/akisatoon1/manaba"
)

var USERNAME = os.Getenv("MANABA_ID")
var PASSWORD = os.Getenv("MANABA_PASS")
var TOKEN = os.Getenv("TOKEN_ERR")

var KADAI_LIST_URL = "https://room.chuo-u.ac.jp/ct/home_library_query"

var DURATION_UNTIL_DEADLINE_STANDARD time.Duration = 48 * time.Hour

type Kadai struct {
	title     string
	titleUrl  string
	course    string
	courseUrl string
	deadline  time.Time
}

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func e(f string, err error) error {
	return fmt.Errorf("%v: %v", f, err.Error())
}

func run() error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return e("cookiejar.New", err)
	}
	err = manaba.Login(jar, USERNAME, PASSWORD)
	if err != nil {
		return e("manaba.Login", err)
	}

	client := makeClient(jar)
	res, err := client.Get(KADAI_LIST_URL)
	if err != nil {
		return e("http.Client.Get", err)
	}
	defer res.Body.Close()
	if c := res.StatusCode; c != 200 {
		return fmt.Errorf("status code is not 200 but %v", c)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return e("goquery.NewDocumentFromReader", err)
	}

	var kadais []Kadai
	err = nil
	format := "2006-01-02 15:04 -07"
	timeDiff := "+09"
	doc.Find("tr[class][class!=title]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		deadline := s.Find("td.center.td-period").Last().Text()
		if deadline == "" {
			return true
		}

		t, er := time.Parse(format, deadline+" "+timeDiff)
		if er != nil {
			err = e("time.Parse", er)
			return false
		}

		if d := time.Until(t); d < DURATION_UNTIL_DEADLINE_STANDARD {
			title, titleUrl, course, courseUrl := getTitleSetAndCourseSet(s)
			kadais = append(kadais, Kadai{title: title, titleUrl: titleUrl, course: course, courseUrl: courseUrl, deadline: t})
		}
		return true
	})
	if err != nil {
		return err
	}

	message := "\n"
	if len(kadais) == 0 {
		message += "直近の課題はありません"
	} else {
		message += "期限が迫っている課題があります\n"
		message += "\n"
		for _, k := range kadais {
			message += fmt.Sprintf("%v\n", k.title)
			message += fmt.Sprintf("(%v)\n", k.course)
			message += "\n"
		}
		message += KADAI_LIST_URL
	}

	err = sendMessage(message, TOKEN)
	if err != nil {
		return e("sendMessage", err)
	}

	return nil
}

func getTitleSetAndCourseSet(s *goquery.Selection) (string, string, string, string) {
	tds := s.Find("td").Not("[class]")
	title, tUrl := getTextAndUrl(tds.First())
	course, cUrl := getTextAndUrl(tds.Last())
	return title, tUrl, course, cUrl
}

func getTextAndUrl(td *goquery.Selection) (string, string) {
	a := td.Find("a")
	u, _ := a.Attr("href")
	return a.Text(), u
}

func makeClient(jar *cookiejar.Jar) *http.Client {
	return &http.Client{Jar: jar}
}

func sendMessage(message string, token string) error {
	url := "https://notify-api.line.me/api/notify"
	body := strings.NewReader("message=" + message)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return fmt.Errorf(fmt.Sprintf("status code is not 200 but %v", res.StatusCode))
	}

	return nil
}
