package containscut

import "strings"

func repoOf(ref string) (string, string, bool) {
	if strings.Contains(ref, "@") { // want "Contains followed by Cut"
		repo, _, _ := strings.Cut(ref, "@")
		return repo, "", true
	}
	return ref, "", false
}

func other(ref string) bool {
	if strings.Contains(ref, "@") {
		_, _, ok := strings.Cut(ref, ":")
		return ok
	}
	return false
}

func withoutContains(ref string) bool {
	_, _, ok := strings.Cut(ref, "@")
	return ok
}
