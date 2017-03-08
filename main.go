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

type Hash string

type Profile struct {
	PeerID        Hash
	Title         string
	Description   string
	Subscriptions map[Hash]bool
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

func download(hash string, w io.Writer) error {
	cmd := exec.Command("ipfs", "cat", hash)
	cmd.Stdout = w
	return cmd.Run()
}

func load(i interface{}, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	return json.NewDecoder(file).Decode(&i)
}

func save(i interface{}, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	return json.NewEncoder(file).Encode(&i)
}

func main() {
	/*peerID, err := whoami()
	if err != nil {
		log.Fatal(err)
	}*/
	var prof Profile
	load(&prof, "profile.json")
	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			r.ParseForm()
			//fmt.Printf("%q", r.Form)
			prof.Title = r.Form.Get("Title")
			prof.Description = r.Form.Get("Description")
			var buff bytes.Buffer
			json.NewEncoder(&buff).Encode(prof)
			hash, err := upload(&buff)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("%q\n", hash)
			//save(prof, "profile.json")
		}
		tpl, err := template.ParseFiles("theme/index.html")
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
		err := download(r.URL.Path, w)
		if err != nil {
			log.Println(err)
		}
	})

	log.Fatal(http.ListenAndServe(":1337", nil))
}
