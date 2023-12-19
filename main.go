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
	"flag"
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaloAltoNetworks/pango"
	"github.com/go-ping/ping"
	"github.com/howeyc/gopass"
)

const (
	version       = "1.2"
	pktCount      = 4 // How many ping packets to send to a host
	allDeviceGrps = "*ALL-DEVICE-GROUPS*"
)

var (
	inputFile    string
	fresh, stale []string // Containers for storing pingable and non-pingable hosts
	deviceGrps   []string
	re           *regexp.Regexp
)

// Represents an address object
type addrObj struct {
	Name  string
	Value string
}

func init() {
	re = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
}
func main() {
	// Print version if requested
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "-v", "--version", "-V", "version":
			fmt.Printf("Pecomm Version: %s\n", version)
			os.Exit(0)
		}
	}
	// Parse Flags
	panoramaNode := flag.String("p", "", "Panorama IP Address (example: -p <panorama_ip/hostname>)")
	flag.StringVar(&inputFile, "f", inputFile, "File to process (example: -f <file_name)")
	flag.Parse()
	if *panoramaNode == "" || inputFile == "" {
		flag.Usage()
		os.Exit(1)
	}
	// Open input file
	fmt.Printf("Attempting to open '%s'...\n", inputFile)
	f, err := os.Open(inputFile)
	handleError(err)
	defer f.Close()
	fmt.Printf("'%s' opened successfully!\n", inputFile)

	// Parse the opened input file
	fmt.Printf("Parsing %s..\n", inputFile)
	var hosts []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		f := re.FindAllString(s.Text(), -1)
		if len(f) != 0 {
			for _, ip := range f {
				addr, err := netip.ParseAddr(ip)
				if err != nil {
					fmt.Println(err)
					continue
				} else {
					hosts = append(hosts, addr.String())
				}
			}
		}
	}
	if len(hosts) == 0 {
		fmt.Fprintln(os.Stderr, fmt.Errorf("error: no hosts found in file '%s'", inputFile))
		os.Exit(1)
	} else {
		fmt.Println("Hosts found within the input file:", hosts)
	}

	// Get the user's credentials
	fmt.Println(`
 ********************************
 *| Enter Panorama Credentials |*
 ********************************`)
	user, pass := getCreds()

	// Create a Panorama client & initialize it
	panor := &pango.Panorama{
		Client: pango.Client{
			Hostname: *panoramaNode,
			Username: user,
			Password: pass,
		},
	}
	if err = panor.Initialize(); err != nil {
		err = fmt.Errorf("unable to connect - ensure you have valid credentials and/or that (%s) is online/valid", *panoramaNode)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
	deviceGrps = append(deviceGrps, "shared")      // <- Add 'shared' device group to the list
	deviceGrps = append(deviceGrps, allDeviceGrps) // <- Add all device groups to the list

	fmt.Println(`
 *******************
 *| Device Groups |*
 *******************`)
	for i, dg := range deviceGrps {
		fmt.Printf("[%d] - %s\n", i, dg)
	}
	fmt.Println("===========================================")
	// Ask user which device group to process against
	var selection string
	input := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Select the device group to process against: ")
		input.Scan()
		selection = input.Text()
		selectionInt, err := strconv.Atoi(selection)
		if err != nil || selectionInt > len(deviceGrps)-1 {
			fmt.Println("Invalid selection")
			continue
		} else {
			break
		}
	}

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

	var foundObjs []addrObj // List of found obj
	for objs := range ch2 {
		foundObjs = append(foundObjs, objs...)
	}
	// If no address objects found for provided IPs (stale), exit
	if len(foundObjs) == 0 {
		fmt.Println("No address objects found for the hosts/servers provided, exiting..")
		os.Exit(0)
	}
	// Collection of address objects found on the Palo based on the provided IPs (stale)
	fmt.Println("**Found objects of hosts (that were unresponsive to pings) that will be removed:")
	for _, obj := range foundObjs {
		fmt.Printf("%+v\n", obj)
	}

	switch pick, _ := strconv.Atoi(selection); deviceGrps[pick] {
	case allDeviceGrps:
		deviceGrps = slices.Delete(deviceGrps, len(deviceGrps)-1, len(deviceGrps))

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
		os.Exit(0)

	default:
		pick, _ := strconv.Atoi(selection)
		dg := deviceGrps[pick]

		// Loop through device group and remove the address object(s) from any address groups
		fmt.Println("**Removing object(s) from any address groups found in the selected device group...")
		for _, fObj := range foundObjs {
			err = removeFromAddrGroups(panor, dg, fObj.Name)
			if err != nil {
				fmt.Fprintln(os.Stderr, fmt.Errorf("address group list error: %w", err))
			}
		}

		// Loop through device group and remove the address object(s) from any security policies
		fmt.Println("**Removing object(s) from security policies found in the selected device group...")
		for _, fObj := range foundObjs {
			err = removeFromSecPolicies(panor, dg, fObj.Name)
			if err != nil {
				fmt.Fprintln(os.Stderr, fmt.Errorf("policy list error: %w", err))
			}
		}

		// Loop through device group and remove the address object(s) from any NAT policies
		fmt.Println("**Removing object(s) from NAT policies found in the selected device group...")
		for _, fObj := range foundObjs {
			err = removeFromNatPolicies(panor, dg, fObj.Name)
			if err != nil {
				fmt.Fprintln(os.Stderr, fmt.Errorf("policy list error: %w", err))
			}
		}

		// Loop through device group and remove the address object(s) themselves
		fmt.Println("**Removing object(s) found in the selected device group...")
		for _, fObj := range foundObjs {
			err = removeAddrObj(panor, dg, fObj.Name)
			if err != nil {
				fmt.Fprintln(os.Stderr, fmt.Errorf("object list error: %w", err))
			}
		}
		fmt.Println(strings.Repeat("*", 88))
		fmt.Println("*** Host(s) Cleanup Process Completed! Don't forget to review and commit the changes.***")
		fmt.Println(strings.Repeat("*", 88))
		os.Exit(0)
	}

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
