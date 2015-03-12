// +build linux,cgo

package main

import (
	"flag"
	"log"

	"gopkg.in/lxc/go-lxc.v2"
)

var (
	lxcpath  string
	name     string
	hostname string
)

func init() {
	flag.StringVar(&lxcpath, "lxcpath", lxc.DefaultConfigPath(), "Use specified container path")
	flag.StringVar(&name, "name", "rubik", "Name of the container")
	flag.StringVar(&hostname, "hostname", "rubik-host1", "Hostname of the container")
	flag.Parse()
}

func main() {

	c, err := lxc.NewContainer(name, lxcpath)
	if err != nil {
		log.Fatalf("ERROR: %s\n", err.Error())
	}

	//setting hostname
	err = c.SetConfigItem("lxc.utsname", hostname)
	if err != nil {
		log.Fatalf("ERROR: %s\n", err.Error())
	}

	// fetching rootfs location
	rootfs := c.ConfigItem("lxc.rootfs")[0]
	log.Printf("Root FS: %s\n", rootfs)

}
