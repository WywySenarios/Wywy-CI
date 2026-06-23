package api

// DenyList maps service names to suite names that should be excluded from
// "All tests" triggering. Entries are populated by handleListServices as the
// deny_list field on each Service response so that the frontend can filter
// them out of the "All tests" trigger. Denied suites remain individually
// triggerable.
var DenyList = map[string]map[string]bool{
	"agentic": {"update-screenshots": true},
}
