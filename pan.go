/*
 * Description: This tool will identify and remove decommissioned hosts/nodes objects and policies from Panorama managed devices.
 * Filename: pan.go
 * Author: Bobby Williams | quipology@gmail.com
 *
 * Copyright (c) 2023
 */
package main

import (
	"fmt"
	"strings"

	"github.com/PaloAltoNetworks/pango"
	"github.com/PaloAltoNetworks/pango/objs/addrgrp"
	"github.com/PaloAltoNetworks/pango/poli/nat"
	"github.com/PaloAltoNetworks/pango/poli/security"
	"github.com/PaloAltoNetworks/pango/util"
)

// This returns a list of a device group's address objects
func getDeviceGrpObjects(p *pango.Panorama, dg string) ([]addrObj, error) {
	var objs []addrObj
	entries, err := p.Objects.Address.GetAll(dg)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		objs = append(objs, addrObj{entry.Name, entry.Value})
	}
	return objs, nil
}

// This finds any objects that represent a host
func findHost(host string, objs []addrObj) (objNames []addrObj) {
	for _, obj := range objs {
		if obj.Value == host {
			objNames = append(objNames, addrObj{obj.Name, host})
			continue
		}
		if strings.Contains(obj.Value, "/") {
			if strings.Split(obj.Value, "/")[0] == host {
				objNames = append(objNames, addrObj{obj.Name, host})
			}
		}
	}
	return
}

// This removes an address object from all of a device group's address groups
func removeFromAddrGroups(p *pango.Panorama, dg, obj string) error {
	entries, err := p.Objects.AddressGroup.GetAll(dg)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		newStaticAddresses := removeFromSlice(entry.StaticAddresses, obj)
		newEntry := addrgrp.Entry{
			Name:            entry.Name,
			Description:     entry.Description,
			StaticAddresses: newStaticAddresses,
			DynamicMatch:    entry.DynamicMatch,
			Tags:            entry.Tags,
		}
		p.Objects.AddressGroup.Edit(dg, newEntry)
	}
	return nil
}

// This removes an object from all a device group's security policies (pre + post)
func removeFromSecPolicies(p *pango.Panorama, dg, obj string) error {
	rulebases := []string{
		util.PreRulebase,
		util.PostRulebase,
	}

	for _, rulebase := range rulebases {
		policies, err := p.Policies.Security.GetAll(dg, rulebase)
		if err != nil {
			return err
		}
		for _, policy := range policies {
			var newPolicy security.Entry
			newPolicy.Copy(policy)
			newPolicy.Name = policy.Name
			newPolicy.Uuid = policy.Uuid
			newPolicy.SourceAddresses = removeFromSlice(newPolicy.SourceAddresses, obj)
			newPolicy.DestinationAddresses = removeFromSlice(newPolicy.DestinationAddresses, obj)
			err = p.Policies.Security.Edit(dg, rulebase, newPolicy)
			if err != nil {
				fmt.Println("Security Policy Edit Error:", err)
			}
		}
	}
	return nil
}

// This removes an object from all device group's NAT policies (pre + post)
func removeFromNatPolicies(p *pango.Panorama, dg, obj string) error {
	rulebases := []string{
		util.PreRulebase,
		util.PostRulebase,
	}

	for _, rulebase := range rulebases {
		policies, err := p.Policies.Nat.GetAll(dg, rulebase)
		if err != nil {
			return err
		}
		for _, policy := range policies {
			var newPolicy nat.Entry
			newPolicy.Copy(policy)
			newPolicy.Name = policy.Name
			newPolicy.Uuid = policy.Uuid
			// These fields must be set inorder to edit the policy
			if newPolicy.Type == "" {
				newPolicy.Type = "ipv4"
			}
			if newPolicy.ToInterface == "" {
				newPolicy.ToInterface = "any"
			}
			if newPolicy.Service == "" {
				newPolicy.Service = "any"
			}
			if newPolicy.SatType == "" {
				newPolicy.SatType = "none"
			}
			if newPolicy.DatType == "" {
				newPolicy.SatType = "none"
			}
			newPolicy.SourceAddresses = removeFromSlice(newPolicy.SourceAddresses, obj)
			newPolicy.DestinationAddresses = removeFromSlice(newPolicy.DestinationAddresses, obj)
			newPolicy.SatTranslatedAddresses = removeFromSlice(newPolicy.SatTranslatedAddresses, obj)
			newPolicy.SatFallbackTranslatedAddresses = removeFromSlice(newPolicy.SatFallbackTranslatedAddresses, obj)
			err = p.Policies.Nat.Edit(dg, rulebase, newPolicy)
			if err != nil {
				fmt.Println("NAT Policy Edit Error:", err)
			}
		}
	}
	return nil
}

// This removes an object from all device groups
func removeAddrObj(p *pango.Panorama, dg, obj string) error {
	objects, err := p.Objects.Address.GetList(dg)
	if err != nil {
		return err
	}

	for _, object := range objects {
		if object == obj {
			err = p.Objects.Address.Delete(dg, object)
			if err != nil {
				fmt.Println("Delete Object Error:", err)
			}
		}
	}
	return nil
}

// For removing item from a slice
func removeFromSlice(s []string, r string) []string {
	newSlice := make([]string, 0)
	m := make(map[string]bool)
	for _, item := range s {
		if item != r {
			if _, exist := m[item]; !exist {
				newSlice = append(newSlice, item)
				m[item] = true
			}
		}
	}
	return newSlice
}
