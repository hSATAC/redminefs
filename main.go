package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	_ "bazil.org/fuse/fs/fstestutil"
	"golang.org/x/net/context"

	"crypto/tls"
	"github.com/mattn/go-redmine"
	"math/rand"

	"path/filepath"
)

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

type config struct {
	Endpoint string `json:"endpoint"`
	Apikey   string `json:"apikey"`
	Project  int    `json:"project"`
	Editor   string `json:"editor"`
	Insecure bool   `json:"insecure"`
}

var profile *string = flag.String("p", os.Getenv("GODMINE_ENV"), "profile")

var conf config

func getConfig() config {
	file := "settings.json"

	if *profile != "" {
		file = "settings." + *profile + ".json"
	}

	if runtime.GOOS == "windows" {
		file = filepath.Join(os.Getenv("APPDATA"), "godmine", file)
	} else {
		file = filepath.Join(os.Getenv("HOME"), ".config", "godmine", file)
	}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		fatal("Failed to read config file: %s\n", err)
	}
	var c config
	err = json.Unmarshal(b, &c)
	if err != nil {
		fatal("Failed to unmarshal file: %s\n", err)
	}
	return c
}

func fatal(format string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, format, err)
	} else {
		fmt.Fprint(os.Stderr, format)
	}
	os.Exit(1)
}

func main() {
	flag.Usage = Usage
	flag.Parse()

	if flag.NArg() != 1 {
		Usage()
		os.Exit(2)
	}

	rand.Seed(time.Now().UnixNano())
	flag.Parse()
	conf = getConfig()
	if conf.Insecure {
		http.DefaultClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	mountpoint := flag.Arg(0)

	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("redmine filesystem"),
		fuse.Subtype("redminefs"),
		fuse.LocalVolume(),
		fuse.VolumeName("Redmine"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	err = fs.Serve(c, FS{})
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}

// FS implements the hello world file system.
type FS struct{}

func (FS) Root() (fs.Node, error) {
	return Projects{}, nil
}

// Project

func (p *Project) Attr(a *fuse.Attr) {
	a.Inode = p.Id
	a.Mode = os.ModeDir | 0555
}

type Issue struct {
	Id int
}

func (i *Issue) Attr(a *fuse.Attr) {
	a.Inode = uint64(i.Id)
	a.Mode = 0444
	a.Size = 1000 // only print the head chunk here.
}

func (i *Issue) ReadAll(ctx context.Context) ([]byte, error) {
	c := redmine.NewClient(conf.Endpoint, conf.Apikey)
	issue, err := c.Issue(i.Id)
	if err != nil {
		fatal("Failed to fetch issue detail: %s\n", err)
	}

	return []byte(issue.Description), nil
}

func (p *Project) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	c := redmine.NewClient(conf.Endpoint, conf.Apikey)
	issues, err := c.IssuesOf(int(p.Id))
	if err != nil {
		fatal("Failed to list issues: %s\n", err)
	}
	var issueList = []fuse.Dirent{}
	for _, i := range issues {
		issueList = append(issueList, fuse.Dirent{Inode: uint64(i.Id), Name: strconv.Itoa(i.Id) + "-" + i.Subject, Type: fuse.DT_File})
	}
	return issueList, nil
}

func (p *Project) Lookup(ctx context.Context, name string) (fs.Node, error) {
	rp := regexp.MustCompile("^[0-9]+")
	str := rp.FindString(name)
	id, err := strconv.Atoi(str)
	if err != nil {
		return nil, fuse.ENOENT
	}
	return &Issue{Id: id}, nil
}

type Projects struct{}

func (Projects) Attr(a *fuse.Attr) {
	a.Inode = 65535
	a.Mode = os.ModeDir | 0555
}

var projects = []fuse.Dirent{}

func (Projects) Lookup(ctx context.Context, name string) (fs.Node, error) {
	for _, i := range projects {
		if name == i.Name {
			return &Project{Id: i.Inode}, nil
		}
	}
	return nil, fuse.ENOENT
}

func (Projects) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	c := redmine.NewClient(conf.Endpoint, conf.Apikey)
	issues, err := c.Projects()
	if err != nil {
		fatal("Failed to list projects: %s\n", err)
		return nil, err
	}

	projects = []fuse.Dirent{}
	for _, i := range issues {
		projects = append(projects, fuse.Dirent{Inode: uint64(i.Id), Name: i.Name, Type: fuse.DT_Dir})
	}
	return projects, nil
}

type Project struct {
	Id uint64
}
