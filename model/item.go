package model

import (
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Canonical struct {
	Href string `json:"href"`
}

type Item struct {
	CrawlTimeMsec string      `json:"crawlTimeMsec"`
	TimestampUsec string      `json:"timestampUsec"`
	Id            string      `json:"id"`
	Categories    []string    `json:"categories"`
	Title         string      `json:"title"`
	Published     int64       `json:"published"`
	Updated       int64       `json:"updated"`
	Starred       int64       `json:"starred"`
	Canonical     []Canonical `json:"canonical"`
	Summary       struct {
		Direction string `json:"direction"`
		Content   string `json:"content"`
	} `json:"summary"`
	Author string `json:"author"`
	Origin struct {
		StreamId string `json:"streamId"`
		Title    string `json:"title"`
		HtmlUrl  string `json:"htmlUrl"`
	} `json:"origin"`
}

func (i *Item) DecodeFields() {
	i.Title = html.UnescapeString(i.Title)
	i.Origin.Title = html.UnescapeString(i.Origin.Title)
}
func (i *Item) PatchedContent() string {
	i.fullBody()
	return fmt.Sprintf("%s %s %s", i.ContentHeader(), i.Summary.Content, i.ContentFooter())
}
func (i *Item) fullBody() {
	uri := i.Link()
	u, err := url.Parse(uri)
	if err != nil {
		log.Printf("cannot parse link %s", uri)
		return
	}

	switch {
	case strings.HasSuffix(u.Host, "sspai.com"):
		full, err := i.grabDoc()
		if err != nil {
			log.Printf("cannot parse content from %s", uri)
			return
		}
		ct := full.Find("div.article-body div.content").First()
		ct.Find("*").RemoveAttr("style").RemoveAttr("class")
		htm, err := ct.Html()
		if err != nil {
			log.Printf("cannot generate content of %s", uri)
			return
		}
		i.Summary.Content = htm
	case strings.HasSuffix(u.Host, "leimao.github.io"):
		full, err := i.grabDoc()
		if err != nil {
			log.Printf("cannot parse content from %s", uri)
			return
		}
		ct := full.Find("article.article div.content").First()
		ct.Find("*").RemoveAttr("style").RemoveAttr("class")
		htm, err := ct.Html()
		if err != nil {
			log.Printf("cannot generate content of %s", uri)
			return
		}
		i.Summary.Content = htm
	case strings.HasSuffix(u.Host, "thoughtworks.cn"):
		full, err := i.grabDoc()
		if err != nil {
			log.Printf("cannot parse content from %s", uri)
			return
		}
		ct := full.Find("article.post div.entry-wrap").First()
		ct.Find("*").RemoveAttr("style").RemoveAttr("class")
		htm, err := ct.Html()
		if err != nil {
			log.Printf("cannot generate content of %s", uri)
			return
		}
		i.Summary.Content = htm
	case strings.HasSuffix(u.Host, "huxiu.com"):
		full, err := i.grabDoc()
		if err != nil {
			log.Printf("cannot parse content from %s", uri)
			return
		}
		js := full.Find("div.js-video-play-log-report-wrap script").Text()
		if js == "" {
			return
		}
		ms := regexp.MustCompile(`'(https://.*video\.huxiucdn\.com/[^']+)'`).FindStringSubmatch(js)
		if len(ms) > 0 {
			tpl := `<video autoplay controls width="100%%"><source src="%s" type="video/mp4"></video>`
			video := fmt.Sprintf(tpl, ms[1])
			i.Summary.Content = video + i.Summary.Content
		}
	}
}
func (i *Item) grabDoc() (doc *goquery.Document, err error) {
	timeout, cancel := context.WithTimeout(context.TODO(), time.Second*15)
	defer cancel()

	req, err := http.NewRequestWithContext(timeout, http.MethodGet, i.Link(), nil)
	if err != nil {
		return
	}
	req.Header.Set("referer", i.Link())
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:94.0) Gecko/20100101 Firefox/94.0")

	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("cannot grab link %s", i.Link())
		return
	}
	defer rsp.Body.Close()
	dat, err := io.ReadAll(rsp.Body)
	if err != nil {
		return
	}
	htm := strings.ReplaceAll(string(dat), "<!--!-->", "")
	return goquery.NewDocumentFromReader(strings.NewReader(htm))
}
func (i *Item) ContentHeader() string {
	const tpl = `
<p>
	<a title="Published: {published}" href="{link}" style="display:block; color: #000; padding-bottom: 10px; text-decoration: none; font-size:1em; font-weight: normal;">
		<span style="display: block; color: #666; font-size:1.0em; font-weight: normal;">{origin}</span>
		<span style="font-size: 1.5em;">{title}</span>
	</a>
</p>`

	replacer := strings.NewReplacer(
		"{link}", i.Link(),
		"{origin}", html.EscapeString(i.Origin.Title),
		"{published}", i.PublishedTime().Format("2006-01-02 15:04:05"),
		"{title}", html.EscapeString(i.Title),
	)

	return replacer.Replace(tpl)
}
func (i *Item) ContentFooter() string {
	const tpl = `
<br/><br/>
<a style="display: block; display: inline-block; border-top: 1px solid #ccc; padding-top: 5px; color: #666; text-decoration: none;"
   href="{link}">{link}</a>
<p style="color:#999;">Save with <a style="color:#666; text-decoration:none; font-weight: bold;" 
									href="https://github.com/gonejack/inostar">inostar</a>
</p>`

	replacer := strings.NewReplacer(
		"{link}", i.Link(),
	)

	return replacer.Replace(tpl)
}
func (i *Item) Link() string {
	if len(i.Canonical) > 0 {
		return i.Canonical[0].Href
	}
	return i.Origin.HtmlUrl
}
func (i *Item) PublishedTime() time.Time {
	return time.Unix(i.Published, 0)
}
