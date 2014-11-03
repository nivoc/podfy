package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"
)

var cfg *Config

type Rss struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"` // required
	RssFeed *RssFeed
}

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
	Image          *FeedImage
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

type FeedImage struct {
	XMLName xml.Name `xml:"image"`
	URL     string   `xml:"url"`
}

type Config struct {
	FeedURL      string `json:"feed_url"`
	FeedTitle    string `json:"feed_title"`
	FeedDesc     string `json:"feed_description"`
	FeedOwner    string `json:"feed_owner"`
	FeedImageURL string `json:"feed_image_url"`
}

func ReadConfig() (*Config, error) {
	homeDir := ""
	usr, err := user.Current()
	if err == nil {
		homeDir = usr.HomeDir
	}
	conf := Config{}
	for _, path := range []string{"/etc/podfy.conf", homeDir + "/.podfy.conf", "./podfy.conf"} {
		file, err := os.Open(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}

		json.NewDecoder(file)
		err = json.NewDecoder(file).Decode(&conf)
		if err != nil {
			return nil, err
		}
		return &conf, nil
	}
	return &conf, nil
}

func ToXML(feed *Rss) (string, error) {
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

func init() {
	var err error

	var feedURL = flag.String("feed_url", "", "the feed url")
	flag.Parse()

	cfg, err = ReadConfig()

	if *feedURL != "" {
		cfg.FeedURL = *feedURL
	}

	if err != nil {
		log.Fatalf("Could not read config: %v", err)
	}
}

func main() {
	//TODO index.html
	http.HandleFunc("/feed.xml", feedHandler)
	http.HandleFunc("/add", addHandler)
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir("./files"))))

	fmt.Println(http.ListenAndServe(":8080", nil))
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	if len(r.Form["auth"]) != 1 {
		fmt.Fprintln(w, "ERR: Auth token missing")
		return
	}
	if r.Form["auth"][0] != "mak" {
		fmt.Fprintln(w, "ERR: Auth token invaild")
		return
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
		Title:          cfg.FeedTitle,
		Link:           cfg.FeedURL,
		Description:    cfg.FeedDesc,
		ManagingEditor: cfg.FeedOwner,
		Image:          &FeedImage{URL: cfg.FeedImageURL},
		// PubDate:     time.Now(),
	}

	feed.Items = []*RssItem{}

	files, _ := ioutil.ReadDir("./files")

	for _, f := range files {
		isMp4 := strings.HasSuffix(f.Name(), ".mp4")
		if !isMp4 {
			continue
		}

		description := ""
		descBuf, err := ioutil.ReadFile("./files/" + f.Name() + ".description")
		fmt.Println(descBuf, err)
		if err == nil {
			description = string(descBuf)
		}

		feed.Items = append(feed.Items, &RssItem{
			Title:       strings.TrimSuffix(f.Name(), ".mp4"),
			Link:        createLink(f.Name()),
			Description: description,
			Enclosure: &RssEnclosure{Url: createLink(f.Name()),
				Length: strconv.Itoa(int(f.Size())),
				Type:   "video/mp4"},
			PubDate: formatTime(f.ModTime()),
		})
	}

	rss20 := &Rss{RssFeed: feed, Version: "2.0"}
	s, _ := ToXML(rss20)

	return s
}

func createLink(filename string) string {
	var Url *url.URL
	Url, err := url.Parse(cfg.FeedURL + "/files/")
	if err != nil {
		panic("boom")
	}

	Url.Path += filename

	fmt.Printf("Encoded URL is %q\n", Url.String())
	return Url.String()
}
