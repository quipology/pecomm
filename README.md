# pecomm v1.2

## What's this?
Pecomm is a firewall cleanup tool that finds and removes host objects from address groups, security policies, NAT policies and the host objects themselves from firewalls managed by Panorama.

## Potential Use Case
Tickets are created in the organization's ticketing system for decommissioned servers in the environment that need to be removed from any related address objects or policies configured on firewalls managed by Panorama.

## Tool Logistics
It will first load a file that is passed in (any file with plain text within it - .txt, .csv, .yaml, etc.) and attempt to ping the loaded hosts. Any hosts that do *NOT* respond to pings will be deemed worthy of removal from the selected device group (or *all* device groups if chosen) managed by a Panorama node. 

Removal happens in this order:

1. Remove found address objects from any address groups (from the device group(s) of your choosing)
2. Remove found address objects from any security policies (from the device group(s) of your choosing)
3. Remove found address objects from any NAT policies (from the device group(s) of your choosing)
4. Remove found address objects (from the device group(s) of your choosing)

## Usage
`pecomm-v1.2-win-amd64.exe -f decommed_servers.txt -p 10.1.2.3`

The `-f` flag is to specify the file that you want pecomm to read and gather IPs from.  
The `-p` flag is to specify the IP/hostname of the Panorama node.  
The `-h` flag is for help.

## In Action
```
PS C:\some_dir> .\release\v1.2\pecomm-v1.2-win-amd64.exe -f tmp.txt -p 10.14.171.3
Attempting to open 'tmp.txt'...
'tmp.txt' opened successfully!
Parsing tmp.txt..
ParseAddr("266.3.3.1"): IPv4 field has value >255
Hosts found within the input file: [8.8.8.8 4.4.4.4 1.1.1.1 7.7.7.7 5.5.5.5 18.1.1.13]

********************************
*| Enter Panorama Credentials |*
********************************
Username: admin
Password: ************
2023/12/9 12:08:27 10.14.171.3: Retrieving API key
Pinging hosts to see if they are online..
8.8.8.8 - is responsive
1.1.1.1 - is responsive
7.7.7.7 - is unresponsive
4.4.4.4 - is unresponsive
5.5.5.5 - is unresponsive
18.1.1.13 - is unresponsive
**Hosts that are ready for removal: [7.7.7.7 4.4.4.4 5.5.5.5 18.1.1.13]

*******************
*| Device Groups |*
*******************
[0] - Test-5-BR-PA850
[1] - VA-PA5220
[2] - BR-PA5220
[3] - FP-PA3220
[4] - CLE-PA5220
[5] - aws-transit
[6] - Security Profiles
[7] - VA-PA-1410-PCI-vsys7
[8] - VA-PA-1410-MF-vsys2
[9] - VA-PA-1410-PRD-vsys4
[10] - VA-PA1410-VAL
[11] - BR-PA1410-PCI-vsys7
[12] - BR-PA1410-Printer-vsys2
[13] - BR-PA1410-PRD-vsys4
[14] - BR-PA1410-VAL
[15] - shared
[16] - *ALL-DEVICE-GROUPS*
===========================================
Select the device group to process against:
```

## Things to Know

- **Changes are not committed by pecomm - you must manually commit and push changes from within Panorama**
- pecomm will NOT remove a host object itself post removing it from all address groups, security & NAT policies if it is associated with any firewall interfaces (this is a good thing)
- Current version only works with ipv4 addresses

### Author
*Bobby Williams | quipology@gmail.com*