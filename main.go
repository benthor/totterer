package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	//	"mime/multipart"
	"encoding/json"
	ipfs "github.com/benthor/totterer/ipfs"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"
)

const VERSION = 0.1
const STATE = "state.json"

type Profile struct {
	PeerID        ipfs.Name
	Title         string
	Description   string
	Subscriptions map[ipfs.Name]bool
}

func hash2interface(addr ipfs.Address, i interface{}) error {
	var buff bytes.Buffer
	err := addr.Download(&buff)
	if err != nil {
		return err
	}
	return json.NewDecoder(&buff).Decode(&i)
}

func interface2hash(i interface{}) (hash ipfs.Hash, err error) {
	var buff bytes.Buffer
	err = json.NewEncoder(&buff).Encode(&i)
	if err != nil {
		return
	}
	return ipfs.Upload(&buff)
}

func Hash2Profile(hash ipfs.Hash) (profile *Profile, err error) {
	err = hash2interface(hash, profile)
	return profile, err
}

func (p *Profile) Hash() (ipfs.Hash, error) {
	return interface2hash(p)

}

type Post struct {
	Type     string // TODO
	Time     time.Time
	Content  ipfs.Hash
	Profile  ipfs.Hash
	Previous ipfs.Hash
	Via      ipfs.Hash
	Version  float64
}

func Hash2Post(hash ipfs.Hash) (post *Post, err error) {
	err = hash2interface(hash, &post)
	return
}

func (p *Post) Hash() (ipfs.Hash, error) {
	return interface2hash(p)
}

type State struct {
	Profile             ipfs.Hash
	LatestPost          ipfs.Hash
	SubscriptionsLatest map[ipfs.Name]ipfs.Hash
}

func LoadState() (state *State, err error) {
	file, err := os.Open(STATE)
	if err != nil {
		log.Println(err)
		peerID, err := ipfs.Whoami()
		if err != nil {
			log.Println(err)
			return nil, err
		}
		prof := Profile{peerID, "Change Me", "Change Me Too", map[ipfs.Name]bool{peerID: true}}
		hash, err := prof.Hash()
		if err != nil {
			return nil, err
		}
		state = &State{hash, "", map[ipfs.Name]ipfs.Hash{}}
		err = state.Save()
		if err != nil {
			return nil, err
		}
	} else {
		err = json.NewDecoder(file).Decode(&state)
	}
	return state, err
}

func (s *State) Save() error {
	file, err := os.Create(STATE)
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println("updating local state")
	return json.NewEncoder(file).Encode(s)
}

func (s *State) NewPost(r io.Reader) error {
	hash, err := ipfs.Upload(r)
	if err != nil {
		log.Println(err)
		return err
	}
	post := Post{"", time.Now(), hash, s.Profile, s.LatestPost, "", VERSION}
	hash, err = post.Hash()
	s.LatestPost = hash
	s.Save()
	_, err = hash.Publish()
	return err
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
		var (
			post *Post
			err  error
		)

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
		if r.URL.Path != "/" {
			post, err = Hash2Post(ipfs.Hash(strings.Trim(r.URL.Path, "/")))
		} else {
			post, err = Hash2Post(state.LatestPost)
		}
		if err != nil {
			log.Println(err)
		}

		tpl, err := template.ParseFiles("theme/post.html")
		if err != nil {
			fmt.Fprintln(w, err)
			log.Fatal(err)
		}
		tpl.Execute(w, post)
	})

	http.HandleFunc("/ipfs/", func(w http.ResponseWriter, r *http.Request) {
		err := ipfs.Hash(r.URL.Path).Download(w)
		if err != nil {
			log.Println(err)
		}
	})

	log.Fatal(http.ListenAndServe("localhost:1337", nil))
}
