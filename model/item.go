package model

import (
	"fmt"
	"html"
	"strings"
	"time"
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
	return fmt.Sprintf("%s %s %s", i.ContentHeader(), i.Summary.Content, i.ContentFooter())
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
