package main

import (
	"testing"
)

func TestParseByteSize_Positive(t *testing.T) {
	items := []struct {
		input    string
		expected int64
	}{
		// bytes
		{"0", 0}, {"123", 123}, {"1000", 1000}, {"1000000", 1000 * 1000}, {"1234567", 1234567},
		// "blocks"
		{"0b", 0}, {"1b", 512}, {"2b", 1024}, {"4b", 2048},
		// killobytes, kibits
		{"1kB", 1000}, {"1KB", 1000}, {"14kB", 14 * 1000}, {"14KB", 14 * 1000},
		{"1024", 1024}, {"1K", 1024}, {"1KiB", 1024}, {"14K", 14 * 1024}, {"14KiB", 14 * 1024},
		// megabytes, mibits
		{"1mB", 1000 * 1000}, {"1MB", 1000 * 1000}, {"24mB", 24 * 1000 * 1000}, {"32MB", 32 * 1000 * 1000},
		{"1M", 1024 * 1024}, {"1MiB", 1024 * 1024}, {"23M", 23 * 1024 * 1024}, {"34MiB", 34 * 1024 * 1024},
		// gigabytes, gibits
		{"1gB", 1000 * 1000 * 1000}, {"1GB", 1000 * 1000 * 1000},
		{"69gB", 69 * 1000 * 1000 * 1000}, {"42GB", 42 * 1000 * 1000 * 1000},
		{"1G", 1024 * 1024 * 1024}, {"1GiB", 1024 * 1024 * 1024},
		{"77G", 77 * 1024 * 1024 * 1024}, {"98GiB", 98 * 1024 * 1024 * 1024},
	}
	for _, item := range items {
		if actual, plus, err := parseByteSize(item.input); err != nil || actual != item.expected || plus {
			if err != nil {
				t.Errorf("Failed parsing %q with: %s\n", item.input, err)
			} else {
				if plus {
					t.Error("Failed for %q: expected plus = false, but got true\n")
				} else {
					t.Errorf("Failed for %q: expected %d, but got %d\n",
						item.input, item.expected, actual)
				}
			}
		}
	}
}

func TestParseByteSize_Errors(t *testing.T) {
	items := []string{"", "+", "b123", "123B", "b", "B", "MB", "1Foo"}
	for _, item := range items {
		if actual, plus, err := parseByteSize(item); err == nil {
			t.Errorf("Expected error for %q, but instead (value: %d / plus: %t)\n", item, actual, plus)
		}
	}
}

func TestParseByteSize_Plus(t *testing.T) {
	items := []struct {
		input    string
		expPlus  bool
		expValue int64
	}{
		{"+0", true, 0},
		{"0", false, 0},
		{"+1K", true, 1024},
		{"1K", false, 1024},
		{"+123456789", true, 123456789},
		{"+1MB", true, 1000 * 1000},
		{"1MB", false, 1000 * 1000},
	}
	for _, item := range items {
		actVal, actPlus, err := parseByteSize(item.input)
		if err != nil {
			t.Errorf("Failed for %q with parse error: %s\n", item.input, err)
		}
		if actPlus != item.expPlus {
			t.Errorf("Failed for %q: Expected plus=%t, but got plus=%t\n",
				item.input, item.expPlus, actPlus)
		}
		if actVal != item.expValue {
			t.Errorf("Failed for %q: Expected value=%d, but got value=%d\n",
				item.input, item.expValue, actVal)
		}
	}
}
