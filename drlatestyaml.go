/*

GoGet
GoFmt
GoBuildNull
GoBuild

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

var (
	DEBUG bool

	KeyPrefix        string
	KeyPrefixReplace string

	RegistryUsername string
	RegistryPassword string
)

func init() {
	if os.Getenv("DEBUG") != "" {
		DEBUG = true
	}

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
	RegistryPassword = os.Getenv("RegistryPassword")
	/*
		if RegistryUsername == "" {
			log("WARNING RegistryUsername env var empty")
		}
		if RegistryPassword == "" {
			log("WARNING RegistryPassword env var empty")
		}
	*/
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
			log("DEBUG %s: %v", imagename, imageurl)
		}
		if imageurl == "" {
			log("WARNING %s value is empty", imagename)
			continue
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
			log("ERROR %s: %v list tags: %v", imagename, imageurl, err)
			os.Exit(1)
		}

		if len(imagetags) > 0 {
			sort.Sort(Versions(imagetags))
			imagetag = imagetags[len(imagetags)-1]
		} else {
			log("ERROR %s: %v no tags", imagename, imageurl)
			os.Exit(1)
		}

		imagenamereplace := KeyPrefixReplace + strings.TrimPrefix(imagename, KeyPrefix)
		tags[imagenamereplace] = imagetag
		if DEBUG {
			log("DEBUG %s: %v tag: %s", imagename, imageurl, imagetag)
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

func log(msg string, args ...interface{}) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, msg+NL)
	} else {
		fmt.Fprintf(os.Stderr, msg+NL, args...)
	}
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
