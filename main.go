package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	//	"mime/multipart"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const VERSION = 0.1
const CONFIG = "config.json"

type Hash string

type Profile struct {
	PeerID        Hash
	Title         string
	Description   string
	Subscriptions map[Hash]bool
}

func Hash2Profile(hash Hash) (*Profile, error) {
	var (
		buff    bytes.Buffer
		profile Profile
	)
	download(hash, &buff)
	err := json.NewDecoder(&buff).Decode(&profile)
	return &profile, err
}

func (p *Profile) Hash() (Hash, error) {
	var (
		buff bytes.Buffer
		hash Hash
	)
	err := json.NewEncoder(&buff).Encode(p)
	if err != nil {
		return hash, err
	}
	return upload(&buff)
}

type Post struct {
	Type     string // TODO
	Time     time.Time
	Content  Hash
	Profile  Hash
	Previous Hash
	Via      Hash
	Version  float64
}

type Config struct {
	Profile    Hash
	LatestPost Hash
}

func LoadConfig() (*Config, error) {
	file, err := os.Open(CONFIG)
	if err != nil {
		log.Println(err)
		peerID, err := whoami()
		if err != nil {
			return nil, err
		}
		prof := Profile{peerID, "Default", "Default", map[Hash]bool{}}
		hash, err := prof.Hash()
		if err != nil {
			return nil, err
		}
		config := Config{hash, peerID}
		err = config.Save()
		if err != nil {
			return nil, err
		}
		return &config, err
	}
	var c Config
	err = json.NewDecoder(file).Decode(&c)
	return &c, err
}

func (c *Config) Save() error {
	file, err := os.Create(CONFIG)
	if err != nil {
		return err
	}
	return json.NewEncoder(file).Encode(c)
}

func Upload(c *Config, r io.Reader) error {
	hash, err := upload(r)
	if err != nil {
		log.Println(err)
		return err
	}
	post := Post{"", time.Now(), hash, c.Profile, c.LatestPost, "", VERSION}
	var buff bytes.Buffer
	json.NewEncoder(&buff).Encode(post)
	hash, err = upload(&buff)
	if err != nil {
		log.Println(err)
		return err
	}
	c.LatestPost = hash

	return nil
}

func upload(r io.Reader) (Hash, error) {
	var (
		out  bytes.Buffer
		hash Hash
		err  error
		re   *regexp.Regexp
	)
	re, err = regexp.Compile("added ([^ ]*)")
	if err != nil {
		return hash, err
	}

	cmd := exec.Command("ipfs", "add", "-")
	cmd.Stdin = r
	cmd.Stdout = &out
	err = cmd.Run()
	if err == nil {
		// FIXME, don't hardcode this
		hash = Hash(re.FindStringSubmatch(out.String())[1])
		//hash = "<a href=\"/ipfs/" + tmp + "\">" + tmp + "</a>"

	}

	return hash, err
}

func whoami() (Hash, error) {
	cmd := exec.Command("ipfs", "config", "Identity.PeerID")
	var (
		out  bytes.Buffer
		hash Hash
	)
	cmd.Stdout = &out
	err := cmd.Run()
	if err == nil {
		hash = Hash(strings.Trim(out.String(), "\n"))
	}
	return hash, err
}

func download(hash Hash, w io.Writer) error {
	cmd := exec.Command("ipfs", "cat", string(hash))
	cmd.Stdout = w
	return cmd.Run()
}

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	prof, err := Hash2Profile(config.Profile)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			r.ParseForm()
			//fmt.Printf("%q", r.Form)
			prof.Title = r.Form.Get("Title")
			prof.Description = r.Form.Get("Description")
			hash, err := prof.Hash()
			if err != nil {
				log.Fatal(err)
			}
			config.Profile = hash
			config.Save()
			//save(prof, "profile.json")
		}
		tpl, err := template.ParseFiles("theme/profile.html")
		if err != nil {
			log.Fatal(err)
		}
		tpl.Execute(w, prof)

	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		res := []string{}
		if r.Method == "POST" {
			reader, err := r.MultipartReader()
			if err != nil {
				log.Println(err)
				res = append(res, err.Error())
			} else {
				for {
					p, err := reader.NextPart()
					if err != nil {
						if err == io.EOF {
							res = append(res, "all done")
							break
						} else {
							log.Println(err)
							res = append(res, err.Error())
							continue
						}
					}
					hash, err := upload(p)
					if err != nil {
						log.Println(err)
						res = append(res, err.Error())
					}
					res = append(res, string(hash))
				}
			}
		}
		message := ""
		if len(res) > 0 {
			message += "<ul><li>" + strings.Join(res, "</li><li>") + "</li><ul>"
		}
		fmt.Fprintf(w, `
<html>
<head></head>
<body>
%s
<form name="upload" method="POST" action="/" enctype="multipart/form-data">
<input type="file" name="fileupload" multiple/>
<input type="submit" value="Go" />
</form>
</body>
</html>
`, message)
	})

	http.HandleFunc("/ipfs/", func(w http.ResponseWriter, r *http.Request) {
		err := download(Hash(r.URL.Path), w)
		if err != nil {
			log.Println(err)
		}
	})

	log.Fatal(http.ListenAndServe(":1337", nil))
}
