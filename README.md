# pecomm v1.0

## What's this?

This tool will find and remove hosts objects from policies (Security & NAT) and the host objects themselves from Panorama managed devices.

It will first load a .csv file with host IPs (in the first column of the .csv - code can be made to ingest different file types) and attempt to ping the loaded hosts. Any hosts that do NOT respond to pings, any found address objects and policies (Sec + NAT) that correlate with unresponsive hosts will be deemed worthy of removal from all firewalls managed by a Panorama node. 

One use case would be if there were tickets opened for decommissioned (hince the name) servers, one may want to remove any related address objects or policies configured on managed Palo firewalls.

Removal happens in this order:

1. Remove found address objects from any address groups (All device groups)
2. Remove found address objects from any security policies (All device groups)
3. Remove found address objects from any NAT policies (All device groups)
4. Remove found address objects (All device groups)

## Things to Know

- **Changes are not committed - you must review and manually commit and push**
- Current version only works with ipv4 addresses