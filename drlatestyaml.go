/*

GoFmt
GoBuildNull
GoBuild

go get -u -a -v
go mod tidy

TODO:
https://pkg.go.dev/github.com/regclient/regclient
https://pkg.go.dev/github.com/google/go-containerregistry
*/

package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/rusenask/docker-registry-client/registry"
	"gopkg.in/yaml.v3"
)

const (
	NL = "\n"
)

func log(msg string, args ...interface{}) {
	const NL = "\n"
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, msg+NL)
	} else {
		fmt.Fprintf(os.Stderr, msg+NL, args...)
	}
}

var (
	DEBUG bool

	KeyPrefix        string
	KeyPrefixReplace string

	RegistryUsername string
	RegistryPassword string
)

func init() {
	KeyPrefix = os.Getenv("KeyPrefix")
	if KeyPrefix == "" {
		log("ERROR KeyPrefix env var empty")
		os.Exit(1)
	}
	KeyPrefixReplace = os.Getenv("KeyPrefixReplace")
	if KeyPrefixReplace == "" {
		log("ERROR KeyPrefixReplace env var empty")
		os.Exit(1)
	}

	RegistryUsername = os.Getenv("RegistryUsername")
	/*
		if RegistryUsername == "" {
			log("WARNING RegistryUsername env var empty")
		}
	*/
	RegistryPassword = os.Getenv("RegistryPassword")
	/*
		if RegistryPassword == "" {
			log("WARNING RegistryPassword env var empty")
		}
	*/
}

type Versions []string

func (vv Versions) Len() int {
	return len(vv)
}

func (vv Versions) Less(i, j int) bool {
	v1, v2 := vv[i], vv[j]
	v1s := strings.Split(v1, ".")
	v2s := strings.Split(v2, ".")
	if len(v1s) < len(v2s) {
		return true
	} else if len(v1s) > len(v2s) {
		return false
	}
	for e := 0; e < len(v1s); e++ {
		d1, _ := strconv.Atoi(v1s[e])
		d2, _ := strconv.Atoi(v2s[e])
		if d1 < d2 {
			return true
		} else if d1 > d2 {
			return false
		}
	}
	return false
}

func (vv Versions) Swap(i, j int) {
	vv[i], vv[j] = vv[j], vv[i]
}

func main() {
	var err error

	flag.Parse()
	if flag.NArg() < 1 {
		log("usage: drlatestyaml /path/to/values.yaml [/path/to/another.values.yaml ...]" + NL +
			"env:" + NL +
			"KeyPrefix" + NL +
			"KeyPrefixReplace" + NL +
			"RegistryUsername" + NL +
			"RegistryPassword" + NL +
			"")
		os.Exit(1)
	}

	fmap := make(map[interface{}]interface{})
	fpaths := flag.Args()[:]
	for _, fpath := range fpaths {
		f, err := os.Open(fpath)
		if err != nil {
			log("ERROR os.Open %v: %v", fpath, err)
			os.Exit(1)
		}
		defer f.Close()

		fdecoder := yaml.NewDecoder(bufio.NewReader(f))
		if err := fdecoder.Decode(&fmap); err != nil {
			log("ERROR yaml.Decoder.Decode %v: %v", fpath, err)
			os.Exit(1)
		}
		f.Close()
	}

	names := make(map[string]string)
	tags := make(map[string]string)

	for k, v := range fmap {
		switch k.(type) {
		case string:
			ks := k.(string)
			if strings.HasPrefix(ks, KeyPrefix) {
				names[ks] = fmt.Sprintf("%s", v)
			}
		}
	}

	for imagename, imageurl := range names {
		if DEBUG {
			log("DEBUG url: %s", imageurl)
		}
		var err error
		imagetag := ""

		if !strings.HasPrefix(imageurl, "https://") && !strings.HasPrefix(imageurl, "http://") && !strings.HasPrefix(imageurl, "oci://") {
			imageurl = fmt.Sprintf("https://%s", imageurl)
		}

		var u *url.URL
		if u, err = url.Parse(imageurl); err != nil {
			log("ERROR %s: %v url parse: %v", imagename, imageurl, err)
			os.Exit(1)
		}

		RegistryUrl := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		//RegistryHost := u.Host
		RegistryRepository := u.Path
		if DEBUG {
			log("DEBUG registry:%s repository:%s", RegistryUrl, RegistryRepository)
		}
		r := registry.NewInsecure(RegistryUrl, RegistryUsername, RegistryPassword)
		r.Logf = registry.Quiet

		imagetags, err := r.Tags(RegistryRepository)
		if err != nil {
			log("WARNING %s: %v list tags: %v", imagename, imageurl, err)
			continue
		}

		sort.Sort(Versions(imagetags))

		if len(imagetags) > 0 {
			imagetag = imagetags[len(imagetags)-1]
		} else {
			imagetag = "latest"
		}

		imagenamereplace := KeyPrefixReplace + strings.TrimPrefix(imagename, KeyPrefix)
		tags[imagenamereplace] = imagetag
		if DEBUG {
			log("DEBUG tag: %s", imagetag)
		}
	}

	if len(tags) > 0 {
		err = yaml.NewEncoder(os.Stdout).Encode(tags)
		if err != nil {
			log("ERROR yaml.Encoder.Encode: %v", err)
			os.Exit(1)
		}
	}

}
