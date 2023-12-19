/*
 * Description: Unit tests for main.go
 * Filename: main_test.go
 * Author: Bobby Williams | quipology@gmail.com
 *
 * Copyright (c) 2023
 */
package main

import (
	"testing"
)

func TestPinger(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"google-dns", "8.8.8.8", true},
		{"some bad IP", "10.10.10.10", false},
		{"another bad IP", "10.254.254.254", false},
		{"google-dns2", "8.8.4.4", true},
	}

	for _, tt := range tests {
		tf := func(t *testing.T) {
			t.Parallel()
			got := pinger(tt.ip)
			if got != tt.want {
				t.Errorf("Expected (%v), but received (%v)\n", tt.want, got)
			}
		}

		t.Run(tt.name, tf)
	}

}
