package timelayout

import "testing"

// TestProbeDiffers guards the element check: a name that printed the same for
// both instants would make check report a layout that names it.
func TestProbeDiffers(t *testing.T) {
	names := []string{
		"2006", "06", "January", "Jan", "01", "1", "Monday", "Mon", "2",
		"_2", "02", "__2", "002", "15", "3", "03", "4", "04", "5", "05",
		"PM", "pm", "MST", "Z0700", "Z07:00", "Z070000", "Z07:00:00",
		"-0700", "-07:00", "-070000", "-07:00:00", "-07", "Z07", ".000",
		".000000", ".000000000", ",000", ",000000", ",000000000", ".9",
		".0", ".00", ".9", ".99", ".999", ".999999999", ",0", ",00", ",9",
		",99", ",999", ",999999", ",999999999",
	}
	for _, name := range names {
		if reference.Format(name) == probe.Format(name) {
			t.Errorf(
				"%q prints %q for both instants",
				name, reference.Format(name),
			)
		}
	}
}
