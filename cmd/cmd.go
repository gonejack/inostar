package cmd

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gonejack/inostar/model"

	"github.com/PuerkitoBio/goquery"
	"github.com/dustin/go-humanize"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
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
		downloads := c.downloadImages(doc)
		doc.Find("img").Each(func(i int, img *goquery.Selection) {
			c.changeRef(img, downloads)
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

	origin := forbiddenCharsReplacer.Replace(item.Origin.Title)
	title := forbiddenCharsReplacer.Replace(item.Title)
	published := item.PublishedTime().Format("2006-01-02 15.04.05")
	target := fmt.Sprintf("[%s][%s][%s].html", origin, published, title)

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

func (c *ConvertStarred) downloadImages(doc *goquery.Document) map[string]string {
	downloadFiles := make(map[string]string)
	downloadLinks := make([]string, 0)
	doc.Find("img").Each(func(i int, img *goquery.Selection) {
		src, _ := img.Attr("src")
		if !strings.HasPrefix(src, "http") {
			return
		}

		localFile, exist := downloadFiles[src]
		if exist {
			return
		}

		uri, err := url.Parse(src)
		if err != nil {
			log.Printf("parse %s fail: %s", src, err)
			return
		}
		localFile = filepath.Join(c.ImagesDir, fmt.Sprintf("%s%s", md5str(src), filepath.Ext(uri.Path)))

		downloadFiles[src] = localFile
		downloadLinks = append(downloadLinks, src)
	})

	var batch = semaphore.NewWeighted(3)
	var group errgroup.Group

	var mutex sync.Mutex
	for i := range downloadLinks {
		_ = batch.Acquire(context.TODO(), 1)

		src := downloadLinks[i]
		group.Go(func() error {
			defer batch.Release(1)

			if c.Verbose {
				log.Printf("fetch %s", src)
			}

			err := c.download(downloadFiles[src], src)
			if err != nil {
				mutex.Lock()
				delete(downloadFiles, src)
				mutex.Unlock()
				log.Printf("download %s fail: %s", src, err)
			}

			return nil
		})
	}

	_ = group.Wait()

	return downloadFiles
}
func (c *ConvertStarred) download(path string, src string) (err error) {
	timeout, cancel := context.WithTimeout(context.TODO(), time.Minute*2)
	defer cancel()

	stat, err := os.Stat(path)
	if err == nil && stat.Size() > 0 {
		headReq, headErr := http.NewRequestWithContext(timeout, http.MethodHead, src, nil)
		if headErr != nil {
			return headErr
		}
		resp, headErr := c.client.Do(headReq)
		if headErr == nil {
			if resp.ContentLength > 0 && resp.ContentLength == stat.Size() {
				return // skip download
			}
		}
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return
	}
	defer file.Close()

	request, err := http.NewRequestWithContext(timeout, http.MethodGet, src, nil)
	if err != nil {
		return
	}
	response, err := c.client.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()

	var written int64
	if c.Verbose {
		bar := progressbar.NewOptions64(response.ContentLength,
			progressbar.OptionSetTheme(progressbar.Theme{Saucer: "=", SaucerPadding: ".", BarStart: "|", BarEnd: "|"}),
			progressbar.OptionSetWidth(10),
			progressbar.OptionSpinnerType(11),
			progressbar.OptionShowBytes(true),
			progressbar.OptionShowCount(),
			progressbar.OptionSetPredictTime(false),
			progressbar.OptionSetDescription(filepath.Base(src)),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionClearOnFinish(),
		)
		defer bar.Clear()
		written, err = io.Copy(io.MultiWriter(file, bar), response.Body)
	} else {
		written, err = io.Copy(file, response.Body)
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("response status code %d invalid", response.StatusCode)
	}

	if err == nil {
		if written == 0 || written < response.ContentLength {
			err = fmt.Errorf("expected %s but downloaded %s", humanize.Bytes(uint64(response.ContentLength)), humanize.Bytes(uint64(written)))
		}
	}

	return
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

var forbiddenCharsReplacer = strings.NewReplacer(
	`<`, `#l`,
	`>`, `#g`,
	`:`, `#c`,
	`"`, `#d`,
	`/`, `#s`,
	`\`, `#b`,
	`|`, `#p`,
	`?`, `#q`,
	`*`, `#a`,
)
