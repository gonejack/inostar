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

func (it *Item) DecodeFields() {
	it.Title = html.UnescapeString(it.Title)
	it.Origin.Title = html.UnescapeString(it.Origin.Title)
}
func (it *Item) PatchedContent() string {
	if it.shouldUnescape(it.Title) {
		it.Title = html.UnescapeString(it.Title)
	}
	if it.shouldUnescape(it.Origin.Title) {
		it.Origin.Title = html.UnescapeString(it.Origin.Title)
	}
	it.fullBody()
	return fmt.Sprintf("%s %s %s", it.ContentHeader(), it.Summary.Content, it.ContentFooter())
}
func (it *Item) fullBody() {
	uri := it.Link()
	u, err := url.Parse(uri)
	if err != nil {
		log.Printf("cannot parse link %s", uri)
		return
	}
	switch {
	case strings.HasSuffix(u.Host, "sspai.com"):
		full, err := it.grabDoc()
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
		it.Summary.Content = htm
	case strings.HasSuffix(u.Host, "leimao.github.io"):
		full, err := it.grabDoc()
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
		it.Summary.Content = htm
	case strings.HasSuffix(u.Host, "thoughtworks.cn"):
		full, err := it.grabDoc()
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
		it.Summary.Content = htm
	case strings.HasSuffix(u.Host, "huxiu.com"):
		full, err := it.grabDoc()
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
			it.Summary.Content = video + it.Summary.Content
		}
	}
}
func (it *Item) grabDoc() (doc *goquery.Document, err error) {
	timeout, cancel := context.WithTimeout(context.TODO(), time.Second*15)
	defer cancel()

	req, err := http.NewRequestWithContext(timeout, http.MethodGet, it.Link(), nil)
	if err != nil {
		return
	}
	req.Header.Set("referer", it.Link())
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:94.0) Gecko/20100101 Firefox/94.0")

	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("cannot grab link %s", it.Link())
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
func (it *Item) ContentHeader() string {
	const tpl = `
<p>
	<a title="Published: {published}" href="{link}" style="display:block; color: #000; padding-bottom: 10px; text-decoration: none; font-size:1em; font-weight: normal;">
		<span style="display: block; color: #666; font-size:1.0em; font-weight: normal;">{origin}</span>
		<span style="font-size: 1.5em;">{title}</span>
	</a>
</p>`

	replacer := strings.NewReplacer(
		"{link}", it.Link(),
		"{origin}", html.EscapeString(it.Origin.Title),
		"{published}", it.PublishedTime().Format("2006-01-02 15:04:05"),
		"{title}", html.EscapeString(it.Title),
	)

	return replacer.Replace(tpl)
}
func (it *Item) ContentFooter() string {
	const tpl = `
<br/><br/>
<a style="display: block; display: inline-block; border-top: 1px solid #ccc; padding-top: 5px; color: #666; text-decoration: none;"
   href="{link}">{link}</a>
<p style="color:#999;">Save with <a style="color:#666; text-decoration:none; font-weight: bold;" 
									href="https://github.com/gonejack/inostar">inostar</a>
</p>`

	replacer := strings.NewReplacer(
		"{link}", it.Link(),
	)

	return replacer.Replace(tpl)
}
func (it *Item) Link() string {
	if len(it.Canonical) > 0 {
		return it.Canonical[0].Href
	}
	return it.Origin.HtmlUrl
}
func (it *Item) PublishedTime() time.Time {
	return time.Unix(it.Published, 0)
}
func (it *Item) shouldUnescape(s string) bool {
	return regexp.MustCompile(`(&#\d{2,6};){2,}`).MatchString(s)
}
