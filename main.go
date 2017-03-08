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
const STATE = "state.json"

type Hash string

type Profile struct {
	PeerID        Hash
	Title         string
	Description   string
	Subscriptions map[Hash]bool
}

func hash2interface(hash Hash, i interface{}) error {
	var buff bytes.Buffer
	err := download(hash, &buff)
	if err != nil {
		return err
	}
	return json.NewDecoder(&buff).Decode(&i)
}

func interface2hash(i interface{}) (Hash, error) {
	var (
		hash Hash
		buff bytes.Buffer
	)
	err := json.NewEncoder(&buff).Encode(&i)
	if err != nil {
		return hash, err
	}
	return upload(&buff)
}

func Hash2Profile(hash Hash) (*Profile, error) {
	var profile Profile
	err := hash2interface(hash, &profile)
	return &profile, err
}

func (p *Profile) Hash() (Hash, error) {
	return interface2hash(p)

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

func Hash2Post(hash Hash) (*Post, error) {
	var post Post
	err := hash2interface(hash, &post)
	return &post, err
}

func (p *Post) Hash() (Hash, error) {
	return interface2hash(p)
}

type State struct {
	Profile    Hash
	LatestPost Hash
}

func LoadState() (*State, error) {
	file, err := os.Open(STATE)
	if err != nil {
		log.Println(err)
		peerID, err := whoami()
		if err != nil {
			log.Println(err)
			return nil, err
		}
		prof := Profile{peerID, "Change Me", "Change Me Too", map[Hash]bool{}}
		hash, err := prof.Hash()
		if err != nil {
			return nil, err
		}
		state := State{hash, peerID}
		err = state.Save()
		if err != nil {
			return nil, err
		}
		return &state, err
	}
	var c State
	err = json.NewDecoder(file).Decode(&c)
	return &c, err
}

func (c *State) Save() error {
	file, err := os.Create(STATE)
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println("updating local state")
	return json.NewEncoder(file).Encode(c)
}

func (c *State) NewPost(r io.Reader) error {
	hash, err := upload(r)
	if err != nil {
		log.Println(err)
		return err
	}
	post := Post{"", time.Now(), hash, c.Profile, c.LatestPost, "", VERSION}
	hash, err = post.Hash()
	if err != nil {
		log.Println(err)
	} else {
		c.LatestPost = hash
		c.Save()
	}
	return err
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

	} else {
		log.Println(err)
	}

	return hash, err
}

func download(hash Hash, w io.Writer) error {
	cmd := exec.Command("ipfs", "cat", string(hash))
	cmd.Stdout = w
	return cmd.Run()
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

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	state, err := LoadState()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		prof, err := Hash2Profile(state.Profile)
		if err != nil {
			log.Fatal(err)
		}
		if r.Method == "POST" {
			r.ParseForm()
			//fmt.Printf("%q", r.Form)
			prof.Title = r.Form.Get("Title")
			prof.Description = r.Form.Get("Description")
			hash, err := prof.Hash()
			if err != nil {
				log.Fatal(err)
			}
			state.Profile = hash
			state.Save()
			//save(prof, "profile.json")
		}
		tpl, err := template.ParseFiles("theme/profile.html")
		if err != nil {
			log.Fatal(err)
		}
		tpl.Execute(w, prof)

	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			reader, err := r.MultipartReader()
			if err != nil {
				log.Println(err)
			} else {
				for {
					p, err := reader.NextPart()
					if err != nil {
						if err == io.EOF {
							err = nil
							break
						} else {
							continue
						}
					}
					err = state.NewPost(p)
					if err != nil {
						log.Println(err)
					}
				}
			}
			if err != nil {
				fmt.Fprintln(w, err)
				log.Fatal(err)
			}
		}
		tpl, err := template.ParseFiles("theme/post.html")
		if err != nil {
			fmt.Fprintln(w, err)
			log.Fatal(err)
		}
		tpl.Execute(w, nil)
	})

	http.HandleFunc("/ipfs/", func(w http.ResponseWriter, r *http.Request) {
		err := download(Hash(r.URL.Path), w)
		if err != nil {
			log.Println(err)
		}
	})

	log.Fatal(http.ListenAndServe(":1337", nil))
}
