/*
 * Description: This tool will identify and remove decommissioned hosts/nodes objects and policies from Panorama managed devices.
 * Filename: main.go
 * Author: Bobby Williams | quipology@gmail.com
 *
 * Copyright (c) 2023
 */
package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PaloAltoNetworks/pango"
	"github.com/go-ping/ping"
	"github.com/howeyc/gopass"
)

const (
	csvFile      = "<some-csv-file>"
	panoramaNode = "<IP/hostname>" // Panorama IP/Hostname
	pktCount     = 4               // How many ping packets to send to a host
)

var (
	fresh, stale []string // Containers for storing pingable and non-pingable hosts
	deviceGrps   []string
)

// Represents an address object
type addrObj struct {
	Name  string
	Value string
}

func main() {
	// Get the user's credentials
	fmt.Println(`
********************************
*| Enter Panorama Credentials |*
********************************`)
	user, pass := getCreds()

	// Create a Panorama client & initialize it
	panor := &pango.Panorama{
		Client: pango.Client{
			Hostname: panoramaNode,
			Username: user,
			Password: pass,
		},
	}
	err := panor.Initialize()
	handleError(err)

	// Open CSV file
	fmt.Printf("Attempting to open '%v'...\n", csvFile)
	f, err := os.Open(csvFile)
	handleError(err)
	defer f.Close()
	fmt.Printf("'%v' opened successfully!\n", csvFile)

	// Parse the opened CSV filef
	fmt.Println("Parsing CSV file..")
	r := csv.NewReader(f)
	l, err := r.ReadAll()
	handleError(err)
	var hosts []string
	for _, host := range l {
		hosts = append(hosts, host[0])
	}
	switch {
	case len(hosts) == 0:
		fmt.Fprintln(os.Stderr, fmt.Errorf("no hosts found in CSV file '%v'", csvFile))
		os.Exit(1)
	default:
		fmt.Println("Hosts found within the CSV file:", hosts)
	}

	// Ping hosts to determine if decommissioned
	fmt.Println("Pinging hosts to see if they are online..")
	var wg sync.WaitGroup
	for i := 0; i < len(hosts); i++ {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			r := pinger(d)
			if !r {
				fmt.Println(d, "- is unresponsive")
				stale = append(stale, d)
				return
			}
			fmt.Println(d, "- is responsive")
			fresh = append(fresh, d)
		}(hosts[i])
	}
	wg.Wait()
	fmt.Println("**Hosts that are ready for removal:", stale)

	// Get a list of all the device groups
	deviceGrps, err = panor.Panorama.DeviceGroup.GetList()
	handleError(err)
	deviceGrps = append(deviceGrps, "shared") // <- Add 'shared' device group to the list

	fmt.Println("**Device Groups Found:", deviceGrps)

	ch1 := make(chan []addrObj, len(deviceGrps))

	// Loop through all the device group's and put their address objects on the channel
	for _, dg := range deviceGrps {
		go func(p *pango.Panorama, dg string) {
			objs, err := getDeviceGrpObjects(p, dg)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				ch1 <- []addrObj{} // Put an empty slice on the channel if error received
			}
			ch1 <- objs
		}(panor, dg)
	}

	// Read all the device group's address objects from the channel and save in a container
	var addrObjs [][]addrObj
	for i := 0; i < len(deviceGrps); i++ {
		addrObjs = append(addrObjs, <-ch1)
	}

	// Look for hosts in any of the address objects (across all device groups)
	ch2 := make(chan []addrObj)

	go func() {
		for _, host := range stale {
			for _, obj := range addrObjs {
				wg.Add(1)
				go func(host string, l []addrObj) {
					defer wg.Done()
					r := findHost(host, l)
					if len(r) != 0 {
						ch2 <- r
					}
				}(host, obj)
			}
		}
		wg.Wait()
		close(ch2)
	}()

	// Collection of address objects found on the Palo based on the stale hosts/servers
	fmt.Println("**Found objects of hosts (that were unresponsive to pings) that will be removed:")
	var foundObjs []addrObj // List of found obj
	for objs := range ch2 {
		fmt.Printf("%+v\n", objs)
		foundObjs = append(foundObjs, objs...)
	}

	// Loop through all device groups and remove the address objects from any address groups
	fmt.Println("**Removing objects from any address groups found across all device groups...")
	for _, dg := range deviceGrps[:len(deviceGrps)-1] {
		for _, fObj := range foundObjs {
			err = removeFromAddrGroups(panor, dg, fObj.Name)
			if err != nil {
				fmt.Fprintln(os.Stderr, fmt.Errorf("address Group List Error: %w", err))
			}
		}
	}

	// Loop through all device groups and remove the address objects from any security policies
	fmt.Println("**Removing objects from security policies found across all device groups...")
	for _, dg := range deviceGrps {
		fmt.Printf("**Processing Device Group: '%v'\n", dg)
		for _, fObj := range foundObjs {
			err = removeFromSecPolicies(panor, dg, fObj.Name)
			if err != nil {
				fmt.Fprintln(os.Stderr, fmt.Errorf("policy list error: %w", err))
			}
		}
	}

	// Loop through all device groups and remove the address objects from any NAT policies
	fmt.Println("**Removing objects from NAT policies found across all device groups...")
	for _, dg := range deviceGrps {
		fmt.Printf("**Processing Device Group: '%v'\n", dg)
		for _, fObj := range foundObjs {
			err = removeFromNatPolicies(panor, dg, fObj.Name)
			if err != nil {
				fmt.Fprintln(os.Stderr, fmt.Errorf("policy list error: %w", err))
			}
		}
	}

	// Loop through all device groups and remove the address objects
	fmt.Println("**Removing objects of all found hosts across all device groups...")
	for _, dg := range deviceGrps {
		fmt.Printf("**Processing Device Group: '%v'\n", dg)
		for _, fObj := range foundObjs {
			err = removeAddrObj(panor, dg, fObj.Name)
			if err != nil {
				fmt.Fprintln(os.Stderr, fmt.Errorf("object list error: %w", err))
			}
		}
	}
	fmt.Println(strings.Repeat("*", 88))
	fmt.Println("*** Host(s) Cleanup Process Completed! Don't forget to review and commit the changes.***")
	fmt.Println(strings.Repeat("*", 88))
}

// Gets credentials from the user
func getCreds() (user, pass string) {
	s := bufio.NewScanner(os.Stdin)
	fmt.Print("Username: ")
	s.Scan()
	user = s.Text()
	p, err := gopass.GetPasswdPrompt("Password: ", true, os.Stdin, os.Stdout)
	handleError(err)
	pass = string(p)
	return
}

// Pings a host to determine if it receives a response
func pinger(dest string) bool {
	p, err := ping.NewPinger(dest)
	handleError(err)
	if runtime.GOOS == "windows" {
		p.SetPrivileged(true)
	}
	p.Count = pktCount
	p.Timeout = 5 * time.Second
	err = p.Run()
	if err != nil {
		return false
	}
	pR := p.Statistics().PacketsRecv
	if pR != 0 {
		return true
	} else {
		return false
	}
}

// For handling non-zero exit errors
func handleError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
