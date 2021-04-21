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
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

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

func (c *ConvertStarred) Execute(jsons []string) error {
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
	doc = c.cleanDoc(doc)

	if c.Offline {
		savedImages := c.saveImages(doc)
		doc.Find("img").Each(func(i int, img *goquery.Selection) {
			c.changeRef(img, savedImages)
		})
	}

	if doc.Find("title").Length() == 0 {
		doc.Find("head").AppendHtml(fmt.Sprintf("<title>%s</title>", html.EscapeString(item.Title)))
	}
	if doc.Find("title").Text() == "" {
		doc.Find("title").SetText(item.Title)
	}

	htm, err := doc.Html()
	if err != nil {
		return fmt.Errorf("cannot generate html: %s", err)
	}

	published := item.PublishedTime().Format("2006-01-02 15.04.05")

	title := item.Title
	if utf8.RuneCountInString(title) > 30 {
		title = string([]rune(title)[:30]) + "..."
	}
	target := fmt.Sprintf("[%s][%s][%s].html", item.Origin.Title, published, title)
	target = safeFileName(target)

	if c.Verbose {
		log.Printf("save %s", target)
	}

	err = ioutil.WriteFile(target, []byte(htm), 0766)
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

	getter := get.DefaultGetter()
	getter.Verbose = c.Verbose
	eRefs, errs := getter.BatchInOrder(refs, paths, 3, time.Minute*2)
	for i := range eRefs {
		log.Printf("download %s fail: %s", eRefs[i], errs[i])
	}

	return downloads
}
func (_ *ConvertStarred) cleanDoc(doc *goquery.Document) *goquery.Document {
	// remove inoreader ads
	doc.Find("body").Find(`div:contains("ads from inoreader")`).Closest("center").Remove()

	// remove solidot.org ads
	doc.Find("img[src='https://img.solidot.org//0/446/liiLIZF8Uh6yM.jpg']").Remove()

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
