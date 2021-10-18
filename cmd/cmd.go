package cmd

import (
	"crypto/md5"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/alecthomas/kong"
	"github.com/gonejack/get"
	"github.com/yosssi/gohtml"

	"github.com/gonejack/inostar/model"
)

type options struct {
	Offline bool `short:"e" help:"Download remote images and replace html references."`
	Verbose bool `short:"v" help:"Verbose printing."`
	About   bool `help:"Show About."`

	JSON []string `arg:"" optional:""`
}
type Convert struct {
	ImagesDir string

	options
	client http.Client
}

func (c *Convert) Run() error {
	kong.Parse(&c.options,
		kong.Name("inostar"),
		kong.Description("Command line tool for converting inoreader starred.json to html"),
		kong.UsageOnError(),
	)
	if c.About {
		fmt.Println("Visit https://github.com/gonejack/inostar")
		return nil
	}
	if len(c.JSON) == 0 {
		return errors.New("no json given")
	}
	if c.Offline {
		err := os.MkdirAll(c.ImagesDir, 0777)
		if err != nil {
			return fmt.Errorf("cannot make images dir %s", err)
		}
	}
	return c.run()
}
func (c *Convert) run() error {
	for _, json := range c.JSON {
		log.Printf("procssing %s", json)

		starred, err := c.openStarred(json)
		if err != nil {
			return err
		}

		err = c.convertStarred(starred)
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *Convert) openStarred(filename string) (*model.Starred, error) {
	fd, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	starred, err := new(model.Starred).FromJSON(fd)
	if err != nil {
		return nil, fmt.Errorf("cannot parse json: %s", err)
	}

	return starred, nil
}
func (c *Convert) convertStarred(starred *model.Starred) (err error) {
	for _, item := range starred.Items {
		log.Printf("processing %s", item.Title)

		err = c.convertItem(item)
		if err != nil {
			return err
		}
	}
	return
}
func (c *Convert) convertItem(item *model.Item) (err error) {
	content := item.PatchedContent()
	content = gohtml.Format(content)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("cannot parse HTML: %s", err)
	}
	doc = c.cleanDoc(doc)

	if doc.Find("title").Length() == 0 {
		doc.Find("head").AppendHtml(fmt.Sprintf("<title>%s</title>", html.EscapeString(item.Title)))
	}
	if doc.Find("title").Text() == "" {
		doc.Find("title").SetText(item.Title)
	}

	if c.Offline {
		downloads := c.saveImages(doc)
		doc.Find("img").Each(func(i int, img *goquery.Selection) {
			c.changeRef(img, downloads)
		})
	}

	pubTime := item.PublishedTime()
	meta := fmt.Sprintf(`<meta name="inostar:publish" content="%s">`, pubTime.Format(time.RFC1123Z))
	doc.Find("head").AppendHtml(meta)

	feedName := fixedLen(item.Origin.Title, 30)
	itemName := fixedLen(item.Title, 30)
	output := safeFilename(fmt.Sprintf("[%s][%s][%s].html", feedName, pubTime.Format("2006-01-02 15.04.05"), itemName))

	if c.Verbose {
		log.Printf("save %s", output)
	}

	htm, err := doc.Html()
	if err != nil {
		return fmt.Errorf("cannot generate html: %s", err)
	}

	err = os.WriteFile(output, []byte(htm), 0666)
	if err != nil {
		return fmt.Errorf("cannot write html: %s", err)
	}

	return
}
func (c *Convert) changeRef(img *goquery.Selection, downloads map[string]string) {
	img.RemoveAttr("loading")
	img.RemoveAttr("srcset")

	src, _ := img.Attr("src")

	switch {
	case strings.HasPrefix(src, "data:"):
		return
	case strings.HasPrefix(src, "http"):
		localFile, exist := downloads[src]
		if !exist {
			log.Printf("localfile for %s not exist", src)
			return
		}

		if c.Verbose {
			log.Printf("replace %s as %s", src, localFile)
		}

		img.SetAttr("data-origin-src", src)
		img.SetAttr("src", localFile)
	default:
		log.Printf("unsupported image reference[src=%s]", src)
	}
}
func (c *Convert) saveImages(doc *goquery.Document) map[string]string {
	downloads := make(map[string]string)
	tasks := get.NewDownloadTasks()

	doc.Find("img").Each(func(i int, img *goquery.Selection) {
		src, _ := img.Attr("src")
		if !strings.HasPrefix(src, "http") {
			return
		}

		localFile, exist := downloads[src]
		if exist {
			return
		}

		uri, err := url.Parse(src)
		if err != nil {
			log.Printf("parse %s fail: %s", src, err)
			return
		}
		localFile = filepath.Join(c.ImagesDir, fmt.Sprintf("%s%s", md5str(src), filepath.Ext(uri.Path)))

		tasks.Add(src, localFile)
		downloads[src] = localFile
	})

	g := get.Default()
	g.OnEachStart = func(t *get.DownloadTask) {
		if c.Verbose {
			log.Printf("downloading %s => %s", t.Link, t.Path)
		}
	}
	g.OnEachStop = func(t *get.DownloadTask) {
		switch {
		case t.Err != nil:
			log.Printf("download %s => %s error: %s", t.Link, t.Path, t.Err)
		case c.Verbose:
			log.Printf("download %s => %s done", t.Link, t.Path)
		}
	}
	g.Batch(tasks, 3, time.Minute*2).ForEach(func(t *get.DownloadTask) {
		if t.Err != nil {
			log.Printf("download %s fail: %s", t.Link, t.Err)
		}
	})

	return downloads
}
func (_ *Convert) cleanDoc(doc *goquery.Document) *goquery.Document {
	// remove inoreader ads
	doc.Find("body").Find(`div:contains("ads from inoreader")`).Closest("center").Remove()

	// remove solidot.org ads
	doc.Find("img[src='https://img.solidot.org//0/446/liiLIZF8Uh6yM.jpg']").Remove()

	// remove 36kr ads
	doc.Find("img[src='https://img.36krcdn.com/20191024/v2_1571894049839_img_jpg']").Closest("p").Remove()

	// remove zaobao ads
	doc.Find("img[src='https://www.zaobao.com.sg/themes/custom/zbsg2020/images/default-img.png']").Closest("p").Remove()

	// remove cnbeta ads
	doc.Find(`strong:contains("访问：")`).Closest("div").Remove()

	// remove empty div
	doc.Find("div:empty").Remove()

	return doc
}

func md5str(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}
func safeFilename(name string) string {
	return regexp.MustCompile(`[<>:"/\\|?*]`).ReplaceAllString(name, ".")
}
func fixedLen(str string, max int) string {
	var rs []rune
	for i, r := range []rune(str) {
		if i >= max {
			if i > 0 {
				rs = append(rs, '.', '.', '.')
			}
			break
		}
		rs = append(rs, r)
	}
	return string(rs)
}
