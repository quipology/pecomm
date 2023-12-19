/*
 * Description: Unit tests for pan.go
 * Filename: pan_test.go
 * Author: Bobby Williams | quipology@gmail.com
 *
 * Copyright (c) 2023
 */
package main

import (
	"testing"
)

func TestRemoveFromSlice(t *testing.T) {
	tests := []struct {
		name          string
		obj           string
		originalSlice []string
		want          int
	}{
		{"test1", "obj1", []string{"obj1", "obj2", "obj3", "obj4"}, 3},
		{"test2", "obj3", []string{"obj1", "obj2", "obj3", "obj4", "obj7"}, 4},
		{"test3", "obj4", []string{"obj1", "obj2", "obj3", "obj4", "obj9", "obj10"}, 5},
	}

	for _, tt := range tests {
		tf := func(t *testing.T) {
			t.Parallel()

			got := removeFromSlice(tt.originalSlice, tt.obj)
			if len(got) != tt.want {
				t.Errorf("Expected (%d), but received (%d)\n", tt.want, len(got))
			}
		}

		t.Run(tt.name, tf)
	}
}

func TestFindHost(t *testing.T) {
	host := "8.8.8.8"
	want := "google-dns-obj"
	objs := []addrObj{
		{"some-other-obj", "10.2.2.2"},
		{"another-obj", "3.2.3.1"},
		{"google-dns-obj", "8.8.8.8"},
		{"test-obj", "172.16.40.12"},
	}

	objNames := findHost(host, objs)
	counter := 0
	for _, obj := range objNames {
		if obj.Name == want {
			counter++
		}
	}
	if counter != 1 {
		t.Errorf("Expected to find object (%s), but found nothing instead\n", want)
	}
}
