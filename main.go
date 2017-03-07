package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	//	"mime/multipart"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
)

type Command struct {
}

func upload(r io.Reader) (string, error) {
	var (
		out  bytes.Buffer
		hash string
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
		tmp := re.FindStringSubmatch(out.String())[1]
		hash = "<a href=\"/ipfs/" + tmp + "\">" + tmp + "</a>"
	}

	return hash, err
}

func download(hash string, w io.Writer) error {
	cmd := exec.Command("ipfs", "cat", hash)
	cmd.Stdout = w
	return cmd.Run()
}

func main() {
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
					res = append(res, hash)
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
