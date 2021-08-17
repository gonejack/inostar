package cmd

import (
	"crypto/md5"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gonejack/get"

	"github.com/gonejack/inostar/model"
)

type ConvertStarred struct {
	client    http.Client
	ImagesDir string

	Offline bool
	Verbose bool
}

func (c *ConvertStarred) Run(jsons []string) error {
	if len(jsons) == 0 {
		return errors.New("no json given")
	}

	if c.Offline {
		err := c.mkdir()
		if err != nil {
			return err
		}
	}

	for _, path := range jsons {
		log.Printf("procssing %s", path)

		starred, err := c.openStarred(path)
		if err != nil {
			return err
		}

		for _, item := range starred.Items {
			log.Printf("processing %s", item.Title)

			err = c.convertItem(item)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *ConvertStarred) openStarred(filename string) (*model.Starred, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %s", err)
	}
	defer file.Close()

	starred, err := new(model.Starred).FromJSON(file)
	if err != nil {
		return nil, fmt.Errorf("cannot parse json: %s", err)
	}

	return starred, nil
}
func (c *ConvertStarred) convertItem(item *model.Item) (err error) {
	content := item.PatchedContent()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("cannot parse HTML: %s", err)
	}
	doc = c.modifyDoc(doc)

	if doc.Find("title").Length() == 0 {
		doc.Find("head").AppendHtml(fmt.Sprintf("<title>%s</title>", html.EscapeString(item.Title)))
	}
	if doc.Find("title").Text() == "" {
		doc.Find("title").SetText(item.Title)
	}

	if c.Offline {
		imageFiles := c.saveImages(doc)
		doc.Find("img").Each(func(i int, img *goquery.Selection) {
			c.changeRef(img, imageFiles)
		})
	}

	pubTime := item.PublishedTime()
	{
		meta := fmt.Sprintf(`<meta name="inostar:publish" content="%s">`, pubTime.Format(time.RFC1123Z))
		doc.Find("head").AppendHtml(meta)
	}
	feedName := safeTitleLen(item.Origin.Title, 30)
	itemName := safeTitleLen(item.Title, 30)
	saving := safeFileName(fmt.Sprintf("[%s][%s][%s].html", feedName, pubTime.Format("2006-01-02 15.04.05"), itemName))

	if c.Verbose {
		log.Printf("save %s", saving)
	}

	htm, err := doc.Html()
	if err != nil {
		return fmt.Errorf("cannot generate html: %s", err)
	}

	err = ioutil.WriteFile(saving, []byte(htm), 0666)
	if err != nil {
		return fmt.Errorf("cannot write html: %s", err)
	}

	return
}
func (c *ConvertStarred) changeRef(img *goquery.Selection, downloads map[string]string) {
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
func (c *ConvertStarred) saveImages(doc *goquery.Document) map[string]string {
	downloads := make(map[string]string)

	var refs, paths []string
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

		refs = append(refs, src)
		paths = append(paths, localFile)
		downloads[src] = localFile
	})

	g := get.DefaultGetter()
	{
		g.BeforeDL = func(ref string, path string) {
			if c.Verbose {
				log.Printf("downloading %s => %s", ref, path)
			}
		}
		g.AfterDL = func(ref string, path string, err error) {
			switch {
			case err != nil:
				log.Printf("download %s => %s error: %s", ref, path, err)
			case c.Verbose:
				log.Printf("download %s => %s done", ref, path)
			}
		}
	}

	eRefs, errs := g.BatchInOrder(refs, paths, 3, time.Minute*2)
	for i := range eRefs {
		log.Printf("download %s fail: %s", eRefs[i], errs[i])
	}

	return downloads
}
func (_ *ConvertStarred) modifyDoc(doc *goquery.Document) *goquery.Document {
	// remove inoreader ads
	doc.Find("body").Find(`div:contains("ads from inoreader")`).Closest("center").Remove()

	// remove solidot.org ads
	doc.Find("img[src='https://img.solidot.org//0/446/liiLIZF8Uh6yM.jpg']").Remove()

	// fix bigboobsjapan.com
	doc.Find("img").Each(func(i int, img *goquery.Selection) {
		src, _ := img.Attr("src")
		if src == "" || !strings.HasPrefix(src, "http") {
			return
		}

		u, err := url.Parse(src)
		if err != nil {
			return
		}

		if u.Host == "www.bigboobsjapan.com" {
			filename := path.Base(u.Path)
			match := regexp.MustCompile(`(.+)-(\d+x\d+)(\.\w{3,4})`).FindStringSubmatch(filename)
			if match == nil {
				return
			}
			u.Path = path.Join(path.Dir(u.Path), match[1]+match[3])
			img.SetAttr("src", u.String())
		}
	})

	return doc
}
func (c *ConvertStarred) mkdir() error {
	err := os.MkdirAll(c.ImagesDir, 0777)
	if err != nil {
		return fmt.Errorf("cannot make images dir %s", err)
	}

	return nil
}

func md5str(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}
func safeFileName(name string) string {
	return regexp.MustCompile(`[<>:"/\\|?*]`).ReplaceAllString(name, ".")
}
func safeTitleLen(title string, max int) string {
	var out []rune
	for i, r := range []rune(title) {
		if i >= max {
			if i > 0 {
				out = append(out, '.', '.', '.')
			}
			break
		}
		out = append(out, r)
	}
	return string(out)
}
