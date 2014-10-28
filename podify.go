package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type RssFeed struct {
	XMLName        xml.Name `xml:"channel"`
	Title          string   `xml:"title"`       // required
	Link           string   `xml:"link"`        // required
	Description    string   `xml:"description"` // required
	Language       string   `xml:"language,omitempty"`
	Copyright      string   `xml:"copyright,omitempty"`
	ManagingEditor string   `xml:"managingEditor,omitempty"` // Author used
	WebMaster      string   `xml:"webMaster,omitempty"`
	PubDate        string   `xml:"pubDate,omitempty"`       // created or updated
	LastBuildDate  string   `xml:"lastBuildDate,omitempty"` // updated used
	Category       string   `xml:"category,omitempty"`
	Generator      string   `xml:"generator,omitempty"`
	Docs           string   `xml:"docs,omitempty"`
	Cloud          string   `xml:"cloud,omitempty"`
	Ttl            int      `xml:"ttl,omitempty"`
	Rating         string   `xml:"rating,omitempty"`
	SkipHours      string   `xml:"skipHours,omitempty"`
	SkipDays       string   `xml:"skipDays,omitempty"`
	Items          []*RssItem
}

type RssItem struct {
	XMLName     xml.Name `xml:"item"`
	Title       string   `xml:"title"`       // required
	Link        string   `xml:"link"`        // required
	Description string   `xml:"description"` // required
	Author      string   `xml:"author,omitempty"`
	Category    string   `xml:"category,omitempty"`
	Comments    string   `xml:"comments,omitempty"`
	Enclosure   *RssEnclosure
	Guid        string `xml:"guid,omitempty"`    // Id used
	PubDate     string `xml:"pubDate,omitempty"` // created or updated
	Source      string `xml:"source,omitempty"`
}

type RssEnclosure struct {
	XMLName xml.Name `xml:"enclosure"`
	Url     string   `xml:"url,attr"`
	Length  string   `xml:"length,attr"`
	Type    string   `xml:"type,attr"`
}

func ToXML(feed *RssFeed) (string, error) {
	data, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return "", err
	}
	// strip empty line from default xml header
	s := xml.Header[:len(xml.Header)-1] + string(data)
	return s, nil
}

func formatTime(t time.Time) string {
	if !t.IsZero() {
		return t.Format(time.RFC822)
	}
	return ""
}

func main() {
	http.HandleFunc("/feed.xml", feedHandler)
	http.HandleFunc("/add", addHandler)
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir("./files"))))

	http.ListenAndServe(":8080", nil)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	if len(r.Form["auth"]) != 1 {
		fmt.Fprintln(w, "ERR: Auth token missing")
		return
	}
	if r.Form["auth"][0] != "mak" {
		fmt.Fprintln(w, "ERR: Auth token invaild")
	}

	url, err := url.Parse(r.Form["url"][0])

	if err != nil {
		log.Fatal(err)
	}
	go executeYoutubeDL(url)
	fmt.Fprintln(w)
}

func executeYoutubeDL(url *url.URL) {
	cmd := exec.Command("youtube-dl", url.String(),
		"--write-info-json", "--write-thumbnail",
		"--write-description", "--restrict-filenames")

	cmd.Dir = "./files"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Wait()
	log.Printf("Command finished with: %v", err)
}

func feedHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, createFeed())
}

func createFeed() string {
	feed := &RssFeed{
		Title:          "jmoiron.net blog",
		Link:           "http://jmoiron.net/blog",
		Description:    "discussion about tech, footie, photos",
		ManagingEditor: "Jason Moiron (jmoiron@jmoiron.net)",
		// PubDate:     time.Now(),
	}

	feed.Items = []*RssItem{}

	files, _ := ioutil.ReadDir("./files")

	for _, f := range files {
		isMp4 := strings.HasSuffix(f.Name(), ".mp4")
		if !isMp4 {
			continue
		}

		feed.Items = append(feed.Items, &RssItem{
			Title:       strings.TrimSuffix(f.Name(), ".mp4"),
			Link:        createLink(f.Name()),
			Description: "",
			Enclosure: &RssEnclosure{Url: createLink(f.Name()),
				Length: strconv.Itoa(int(f.Size())),
				Type:   "video/mp4"},
			PubDate: formatTime(f.ModTime()),
		})
	}

	s, _ := ToXML(feed)

	return s
}

func createLink(filename string) string {
	var Url *url.URL
	Url, err := url.Parse("http://192.168.2.101:8080/files/")
	if err != nil {
		panic("boom")
	}

	Url.Path += filename

	fmt.Printf("Encoded URL is %q\n", Url.String())
	return Url.String()
}
