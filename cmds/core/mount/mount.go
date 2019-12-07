// Copyright 2012-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Mount a filesystem at the specified path.
//
// Synopsis:
//     mount [-r] [-o options] [-t FSTYPE] DEV PATH
//
// Options:
//     -r: read only
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/u-root/u-root/pkg/loop"
	"github.com/u-root/u-root/pkg/mount"
	"golang.org/x/sys/unix"
)

type mountOptions []string

var nfsre = regexp.MustCompile(`^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):[/\w]+$`)

func (o *mountOptions) String() string {
	return strings.Join(*o, ",")
}

func (o *mountOptions) Set(value string) error {
	for _, option := range strings.Split(value, ",") {
		*o = append(*o, option)
	}
	return nil
}

var (
	ro      = flag.Bool("r", false, "Read only mount")
	fsType  = flag.String("t", "", "File system type")
	options mountOptions
)

func init() {
	flag.Var(&options, "o", "Comma separated list of mount options")
}

func loopSetup(filename string) (loopDevice string, err error) {
	loopDevice, err = loop.FindDevice()
	if err != nil {
		return "", err
	}
	if err := loop.SetFile(loopDevice, filename); err != nil {
		return "", err
	}
	return loopDevice, nil
}

// extended from boot.go
func getSupportedFilesystem(originFS string) ([]string, bool, error) {
	var known bool
	var err error
	fs, err := ioutil.ReadFile("/proc/filesystems")
	if err != nil {
		return nil, known, err
	}
	var returnValue []string
	for _, f := range strings.Split(string(fs), "\n") {
		n := strings.Fields(f)
		last := len(n) - 1
		if last < 0 {
			continue
		}
		if n[last] == originFS {
			known = true
		}
		returnValue = append(returnValue, n[last])
	}
	return returnValue, known, err

}

func informIfUnknownFS(originFS string) {
	knownFS, known, err := getSupportedFilesystem(originFS)
	if err != nil {
		// just don't make things even worse...
		return
	}
	if !known {
		log.Printf("Hint: unknown filesystem %s. Known are: %v", originFS, knownFS)
	}
}

func printMounts() error {
	mts, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		return err
	}
	fmt.Printf("%s", mts)
	return nil
}

func main() {
	flag.Parse()
	a := flag.Args()

	if flag.NArg()+flag.NFlag() == 0 {
		printMounts()
		os.Exit(0)
	}

	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	dev := a[0]
	path := a[1]
	var flags uintptr
	var data []string
	var err error
	for _, option := range options {
		switch option {
		case "loop":
			dev, err = loopSetup(dev)
			if err != nil {
				log.Fatal("Error setting loop device:", err)
			}
		default:
			if f, ok := opts[option]; ok {
				flags |= f
			} else {
				data = append(data, option)
			}
		}
	}
	if *ro {
		flags |= unix.MS_RDONLY
	}
	if *fsType == "" {
		// mandatory parameter for the moment
		log.Fatalf("No file system type provided!\nUsage: mount [-r] [-o mount options] -t fstype dev path")
	}
	if *fsType == "nfs" || *fsType == "nfs3" || *fsType == "nfs4" {
		// deal with <ip>:<mntpt> syntax
		match := nfsre.FindAllStringSubmatch(dev, -1)
		if len(match) > 0 && len(match[0]) > 1 {
			data = append(data, fmt.Sprintf("addr=%s", match[0][1]))
		}
	}
	if err := mount.Mount(dev, path, *fsType, strings.Join(data, ","), flags); err != nil {
		log.Printf("%v", err)
		informIfUnknownFS(*fsType)
		os.Exit(1)
	}
}
