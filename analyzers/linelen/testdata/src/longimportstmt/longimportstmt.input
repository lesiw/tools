package longimports

import averylongpackagenamethatexceedsthecharacterlimitbutwhoseimportshouldnttriggeranalyzer "io"

func _() {
	_ = averylongpackagenamethatexceedsthecharacterlimitbutwhoseimportshouldnttriggeranalyzer.EOF // want "line is 153 characters long, exceeds 79 limit"
}
