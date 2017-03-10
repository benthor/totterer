package ipfs

import (
	"bytes"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

type Address interface {
	Resolve() (path string, err error)
	Download(w io.Writer) (err error)
	String() string
}

func download(a Address, w io.Writer) (err error) {
	path, err := a.Resolve()
	if err != nil {
		return err
	}
	cmd := exec.Command("ipfs", "cat", path)
	cmd.Stdout = w
	return cmd.Run()
}

func resolve(cmd *exec.Cmd) (path string, err error) {
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	path = strings.Trim(out.String(), " \n")
	return
}

type Hash string

func Upload(r io.Reader) (hash Hash, err error) {
	cmd := exec.Command("ipfs", "add", "-q")
	var out bytes.Buffer
	cmd.Stdin = r
	cmd.Stdout = &out
	err = cmd.Run()
	hash = Hash(strings.Trim(out.String(), " \n"))
	return
}

func (h Hash) String() string {
	return string(h)
}

func (h Hash) Resolve() (path string, err error) {
	cmd := exec.Command("ipfs", "resolve", h.String())
	return resolve(cmd)
}

func (h Hash) Publish() (name Name, err error) {
	re := regexp.MustCompile("Published to ([^:]*):")
	cmd := exec.Command("ipfs", "name", "publish", h.String())
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	return Name(re.FindStringSubmatch(out.String())[1]), err
}

func (h Hash) Download(w io.Writer) (err error) {
	return download(h, w)
}

type Name string

func Whoami() (n Name, err error) {
	cmd := exec.Command("ipfs", "config", "Identity.PeerID")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err == nil {
		n = Name(strings.Trim(out.String(), "\n"))
	}
	return n, err
}

func (n Name) String() string {
	return string(n)
}

func (n Name) Resolve() (path string, err error) {
	cmd := exec.Command("ipfs", "name", "resolve", n.String())
	return resolve(cmd)
}

func (n Name) Download(w io.Writer) (err error) {
	return download(n, w)
}
