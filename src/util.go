package main

import "strings"

func toCamelCase(input string) string {
	parts := strings.Split(input, "-")
	for i, part := range parts {
		if i > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

func contains(a []string, s string) bool {
	for _, e := range a {
		if e == s {
			return true
		}
	}
	return false
}
